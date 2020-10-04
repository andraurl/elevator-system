package synchronization

import (
	"fmt"
	"time"

	"../config"
	"../elevio"
	"../esm"
	"../network/peers"
)

// Channels are channels used by Syncrhonization to communiate with other modules.
type Channels struct {
	IncomingMsg             chan esm.ElevData
	OutgoingMsg             chan esm.ElevData
	SyncedElevData          chan []esm.ElevData
	LocalElevData           chan esm.ElevData
	TransmitEnable          chan bool
	PeerUpdateCh            chan peers.PeerUpdate
	CompletedOrder          chan elevio.ButtonEvent
	HallOrder               chan elevio.ButtonEvent
	ClearedOrderStatusOrder chan elevio.ButtonEvent
}

// Synchronize is the function that continoulsy synchronize the data between
// the contributing elevators and pass the needed information to the rest of
// the local system on each elevator.
func Synchronize(channels Channels, myID string) {
	elevData := make([]esm.ElevData, 3)

	initializedOrderStatus := make([][][]int, config.MaxNumElevators)
	initializedLocalQueue := make([][][]int, config.MaxNumElevators)
	for j := range initializedOrderStatus {
		initializedOrderStatus[j] = make([][]int, config.NumButtonTypes)
		initializedLocalQueue[j] = make([][]int, config.NumButtonTypes)
		for k := range initializedOrderStatus[j] {
			initializedOrderStatus[j][k] = make([]int, config.NumFloors)
			initializedLocalQueue[j][k] = make([]int, config.NumFloors)
		}
	}

	for i := 0; i < config.MaxNumElevators; i++ {
		elevData[i] = esm.ElevData{
			OrderStatus: initializedOrderStatus[i],
			LocalQueue:  initializedLocalQueue[i],
			Online:      false,
		}
	}
	elevData[0].ID = myID

	sendOutgoinUpdateTimer := time.NewTimer(time.Millisecond * 100)
	sendCopyToDist := make(chan bool)
	OrderStatusUpdate := make(chan bool)

	for {
		select {

		case update := <-channels.PeerUpdateCh:

			fmt.Println("Received peer update msg")
			fmt.Printf("Peer update:\n")
			fmt.Printf("  Peers:    %q\n", update.Peers)
			fmt.Printf("  New:      %q\n", update.New)
			fmt.Printf("  Lost:     %q\n", update.Lost)

			if update.New != "" {
				found := false
				for i, elev := range elevData {
					if update.New == elev.ID {
						found = true
						elevData[i].Online = true
					}
				}
				if !found {
					for i, elev := range elevData {
						if elev.ID == "" {
							elevData[i].ID = update.New
							elevData[i].Online = true
							break
						}
					}
				}
			}
			for _, lostID := range update.Lost {
				for i, elev := range elevData {
					if elev.ID == lostID {
						elevData[i].Online = false
					}
				}
			}
			go func() { sendCopyToDist <- true }()

		case <-sendOutgoinUpdateTimer.C:
			sendOutgoinUpdateTimer.Reset(time.Millisecond * 100)
			channels.OutgoingMsg <- elevData[0]

		case shallowElevUpdate := <-channels.IncomingMsg:
			var elevUpdate esm.ElevData

			esm.DeepCopy(&elevUpdate, &shallowElevUpdate)
			if elevUpdate.ID == myID {
				break
			}
			for i, elev := range elevData {
				if elev.ID == elevUpdate.ID &&
					!hasPeerChange(elev, elevUpdate) {
					elevData[i].State = elevUpdate.State
					elevData[i].HeadingDir = elevUpdate.HeadingDir
					elevData[i].Floor = elevUpdate.Floor
					elevData[i].OrderStatus = elevUpdate.OrderStatus
					elevData[i].LocalQueue = elevUpdate.LocalQueue
					go func() { sendCopyToDist <- true }() // check if we need this
					go func() { OrderStatusUpdate <- true }()
				}

			}

		case <-sendCopyToDist:
			copyData := make([]esm.ElevData, config.MaxNumElevators)
			esm.DeepCopy(&copyData, &elevData)
			channels.SyncedElevData <- copyData

		case order := <-channels.CompletedOrder:
			println("Receiving completed order")
			fmt.Println(elevData[0].OrderStatus)
			if elevData[0].OrderStatus[int(order.Button)][order.Floor] == 1 {
				elevData[0].OrderStatus[int(order.Button)][order.Floor] = -1
				println("Updated OrderStatus to -1")
				fmt.Println(elevData[0].OrderStatus)
				//go func() { sendCopyToDist <- true }()
				go func() { OrderStatusUpdate <- true }()
			}

		case order := <-channels.HallOrder:
			if elevData[0].OrderStatus[int(order.Button)][order.Floor] == 0 {
				elevData[0].OrderStatus[int(order.Button)][order.Floor] = 1
			}
			go func() { OrderStatusUpdate <- true }()

		case shallowElevUpdate := <-channels.LocalElevData:
			var elevUpdate esm.ElevData
			esm.DeepCopy(&elevUpdate, &shallowElevUpdate)

			if !hasLocalUpdate(elevData[0], elevUpdate) {
				elevData[0].State = elevUpdate.State
				elevData[0].HeadingDir = elevUpdate.HeadingDir
				elevData[0].Floor = elevUpdate.Floor
				elevData[0].LocalQueue = elevUpdate.LocalQueue

				go func() { sendCopyToDist <- true }()
			}

		case <-OrderStatusUpdate:
			println("Recieved OrderStatusUpdate")
			fmt.Println(elevData[0].OrderStatus)

			originalElevData := make([]esm.ElevData, config.MaxNumElevators)
			esm.DeepCopy(&originalElevData, &elevData)

			for buttonNr := 0; buttonNr < config.NumButtonTypes; buttonNr++ {
				for floorNr := 0; floorNr < config.NumFloors; floorNr++ {

					switch originalElevData[0].OrderStatus[buttonNr][floorNr] {
					case 0:
						for i := 1; i < config.MaxNumElevators; i++ {
							if originalElevData[i].OrderStatus[buttonNr][floorNr] == 1 {
								elevData[0].OrderStatus[buttonNr][floorNr] = 1
							}
						}

					case 1:
						for i := 1; i < config.MaxNumElevators; i++ {
							if originalElevData[i].OrderStatus[buttonNr][floorNr] == -1 {
								elevData[0].OrderStatus[buttonNr][floorNr] = -1
							}
						}

					case -1:
						notOne := true
						for i := 1; i < config.MaxNumElevators; i++ {
							if originalElevData[i].Online && elevData[i].OrderStatus[buttonNr][floorNr] == 1 {
								notOne = false
							}
						}
						if isAlone(originalElevData) || notOne {
							elevData[0].OrderStatus[buttonNr][floorNr] = 0
							order := elevio.MakeButtonEvent(buttonNr, floorNr)
							go func() { channels.ClearedOrderStatusOrder <- order }()

						}
					}
				}
			}
			if hasPeerChange(originalElevData[0], elevData[0]) {
				go func() { sendCopyToDist <- true }()
			}
		}
	}
}
