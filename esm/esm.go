package esm

import (
	"fmt"
	"os"
	"time"

	"../config"
	elevio "../elevio"
)

// ElevState defines the state of the elevator.
type ElevState int

const (
	//Undefined state is the default elevator state.
	Undefined ElevState = iota - 1
	//Idle state defines an elevator state while at rest with door closed.
	Idle
	//Moving defines an elevators state while moving.
	Moving
	//DoorOpen defines an elevators state while at rest with the door open.
	DoorOpen
)

//HeadingDirection defines the moving direction of an elevator
type HeadingDirection int

const (
	//HeadingUp defines an elevator moving upwards
	HeadingUp = 1
	//HeadingDown defines an elevator moving downwards
	HeadingDown = -1
)

//ElevData defines an elevators state, direction, floor, order list
//configuration and elevator state machine channles.
type ElevData struct {
	ID          string
	State       ElevState
	HeadingDir  HeadingDirection
	Floor       int
	OrderStatus [][]int
	LocalQueue  [][]int
	Online      bool
}

// Channels are channels used by esm to communiate with other modules.
type Channels struct {
	NewOrder        chan elevio.ButtonEvent
	ArrivedAtFloor  chan int
	WatchDogTimeOut chan bool
	TurnOnLight     chan elevio.ButtonEvent
	TurnOffLight    chan elevio.ButtonEvent
	CompletedOrder  chan elevio.ButtonEvent
	LocalElevData   chan ElevData
}

//ESM is state machine for completing given orders
func ESM(channels Channels, initFloor int) {

	localQueue := make([][]int, config.NumButtonTypes)
	for i := range localQueue {
		localQueue[i] = make([]int, config.NumFloors)
	}

	elevator := ElevData{
		State:      Idle,
		HeadingDir: HeadingUp,
		Floor:      initFloor,
		LocalQueue: localQueue,
	}

	doorTimer := time.NewTimer(config.DoorTimerDuration * time.Second)
	doorTimer.Stop()
	watchDogTimer := time.NewTimer(config.WatchDogTimerDuration * time.Second)
	watchDogTimer.Stop()
	motorLossTimer := time.NewTimer(config.MotorLossTimerDuration * time.Second)
	motorLossTimer.Stop()

	// Local channels
	closeDoor := make(chan bool)
	openDoor := make(chan bool)
	sendLocalData := make(chan bool)

	backup := readBackupQueue()
	for i := 0; i < config.NumButtonTypes; i++ {
		for k := 0; k < config.NumFloors; k++ {
			if backup[i][k] == 1 {
				order := elevio.MakeButtonEvent(i, k)
				go func() {
					channels.TurnOnLight <- order
					channels.NewOrder <- order
				}()
			}
		}
	}
	backupFile, _ := os.Create("esm/order_backup.txt")
	backupFile.Truncate(12)
	defer backupFile.Close()

	go func() { sendLocalData <- true }()

	for {
		select {
		case newOrder := <-channels.NewOrder:
			fmt.Printf("Recieved new order: %+v\n", newOrder)
			elevator.LocalQueue = addOrderToQueue(elevator.LocalQueue, newOrder)
			backupQueue(backupFile, elevator.LocalQueue)
			switch elevator.State {
			case Idle:
				if hasOrderAtCurrentFloor(elevator) {
					go func() { openDoor <- true }()
				} else {
					elevator.HeadingDir = chooseHeadingDirection(elevator)
					elevio.SetMotorDirection(getMotorDirection(elevator.HeadingDir))
					elevator.State = Moving
					watchDogTimer.Stop()
					watchDogTimer.Reset(config.WatchDogTimerDuration * time.Second)
					motorLossTimer.Stop()
					motorLossTimer.Reset(time.Second * config.MotorLossTimerDuration)
					fmt.Println("Elevator state : Moving")
				}
			case DoorOpen:
				if shouldStop(elevator) {
					go func() { openDoor <- true }()
				}
			default:

			}
			go func() { sendLocalData <- true }()

		case elevator.Floor = <-channels.ArrivedAtFloor:
			elevio.SetFloorIndicator(elevator.Floor)
			motorLossTimer.Stop()
			motorLossTimer.Reset(time.Second * config.MotorLossTimerDuration)

			if elevator.Floor == config.NumFloors-1 {
				elevator.HeadingDir = HeadingDown
			}
			if elevator.Floor == 0 {
				elevator.HeadingDir = HeadingUp
			}
			if elevator.State == Undefined {
				elevator.State = Moving
				motorLossTimer.Stop()
				motorLossTimer.Reset(time.Second * config.MotorLossTimerDuration)
			}

			if shouldStop(elevator) {
				go func() { openDoor <- true }()
			}
			go func() { sendLocalData <- true }()

		case <-closeDoor:
			elevator.HeadingDir = chooseHeadingDirection(elevator)
			if shouldStop(elevator) && hasOrderAtCurrentFloor(elevator) {
				go func() { openDoor <- true }()
				break
			}
			fmt.Println("Closing door ")
			watchDogTimer.Stop()
			watchDogTimer.Reset(config.WatchDogTimerDuration * time.Second)
			elevio.SetDoorOpenLamp(false)

			if !shouldStop(elevator) {
				elevator.State = Moving
				elevio.SetMotorDirection(getMotorDirection(elevator.HeadingDir))
				motorLossTimer.Stop()
				motorLossTimer.Reset(time.Second * config.MotorLossTimerDuration)
			} else {
				elevator.State = Idle
			}
			go func() { sendLocalData <- true }()

		case <-openDoor:
			fmt.Println("Open door ")
			elevator.State = DoorOpen
			elevio.SetMotorDirection(elevio.MD_Stop)
			elevio.SetDoorOpenLamp(true)

			for i := 0; i < config.NumButtonTypes; i++ {
				order := elevio.MakeButtonEvent(i, elevator.Floor)
				if shouldClearOrder(elevator, order) {
					println("Sending completed order: Button: ", order.Button, " Floor:", order.Floor)
					elevator.LocalQueue[order.Button][order.Floor] = 0
					go func() { channels.CompletedOrder <- order }()
				}
			}
			elevator.LocalQueue[elevio.BT_Cab][elevator.Floor] = 0
			turnOffLightsOfClearedOrders(elevator)
			backupQueue(backupFile, elevator.LocalQueue)

			doorTimer.Reset(config.DoorTimerDuration * time.Second)
			watchDogTimer.Stop()
			watchDogTimer.Reset(config.WatchDogTimerDuration * time.Second)

			motorLossTimer.Stop()
			go func() { sendLocalData <- true }()

		case <-doorTimer.C:
			fmt.Println("Door Timeout")
			go func() { closeDoor <- true }()

		case <-watchDogTimer.C:
			fmt.Println("ESM: WatchDogTimer Timeout")

			watchDogTimer.Reset(time.Second * config.WatchDogTimerDuration)
			channels.WatchDogTimeOut <- true

		case <-motorLossTimer.C:
			println("MotorLosstimer Timeout.\nelevator.State = Undefined")
			elevator.State = Undefined
			go func() { sendLocalData <- true }()

		case <-sendLocalData:
			var copyData ElevData
			DeepCopy(&copyData, &elevator)
			fmt.Println("Sending localData from esm: ", copyData)
			go func() { channels.LocalElevData <- copyData }()

		case order := <-channels.TurnOnLight:
			elevio.SetButtonLamp(order.Button, order.Floor, true)

		case order := <-channels.TurnOffLight:
			elevio.SetButtonLamp(order.Button, order.Floor, false)

		default:
		}
	}
}
