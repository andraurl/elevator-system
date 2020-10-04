package esm

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"strconv"

	"../config"
	"../elevio"
)

//DeepCopy perfoms deepCopy of source into target.
func DeepCopy(target, source interface{}) {
	n, _ := json.Marshal(source)
	json.Unmarshal(n, target)
}

func readBackupQueue() [][]int {
	queue := make([][]int, config.NumButtonTypes)
	for i := range queue {
		queue[i] = make([]int, config.NumFloors)
	}

	//backup is previous elevator.LocalQueue saved from file
	backup, err := ioutil.ReadFile("esm/order_backup.txt")
	if err != nil {
		println("esm: Could not read from file. Creating new file.")
		os.Create("esm/order_backup.txt")
	} else {
		println("esm: Read from file: ", string(backup))
		// If backup is formatted correctly we add it to backupQueue
		if len(string(backup)) == config.NumFloors*config.NumButtonTypes+1 || len(string(backup)) == config.NumFloors*config.NumButtonTypes {
			println("String length correct")
			for i := 0; i < config.NumButtonTypes; i++ {
				for k := 0; k < config.NumFloors; k++ {
					isOrder, _ := strconv.Atoi(string(backup[i*config.NumFloors+k]))
					if isOrder == 1 || isOrder == 0 {
						queue[i][k] = isOrder
					}
				}
			}
		}
	}
	return queue
}

func backupQueue(file *os.File, queue [][]int) {
	queueString := ""
	file.Seek(0, 0)

	for i := range queue {
		for k := range queue[i] {
			order := strconv.Itoa(queue[i][k])
			queueString += order
		}
	}
	//println("Backing up queue: ", queueString)
	//n, _ := file.WriteString(queueString)
	file.WriteString(queueString)
	//println("Backed up ", n, "bytes of data")
}

func chooseHeadingDirection(elev ElevData) HeadingDirection {
	switch elev.HeadingDir {
	case HeadingUp:
		if hasOrderAbove(elev) ||
			hasHallUpOrderAtCurrentFloor(elev) ||
			elev.Floor == 0 {
			return HeadingUp
		}
		return HeadingDown
	case HeadingDown:
		if HasOrderBelow(elev) ||
			hasHallDownOrderAtCurrentFloor(elev) ||
			elev.Floor == config.NumFloors-1 {
			return HeadingDown
		}
		return HeadingUp

	default:
	}
	return HeadingUp
}

func shouldStop(elev ElevData) bool {
	switch elev.HeadingDir {
	case HeadingUp:
		if hasCabOrderAtCurrentFloor(elev) ||
			hasHallUpOrderAtCurrentFloor(elev) ||
			!hasOrderAbove(elev) {
			return true
		}
	case HeadingDown:
		if hasCabOrderAtCurrentFloor(elev) ||
			hasHallDownOrderAtCurrentFloor(elev) ||
			!HasOrderBelow(elev) {
			return true
		}
	default:
	}
	return false
}

//hasOrderAbove returns whether an elev has a local order above last known floor
func hasOrderAbove(elev ElevData) bool {
	for floor := elev.Floor + 1; floor < config.NumFloors; floor++ {
		for button := 0; button < config.NumButtonTypes; button++ {
			if elev.LocalQueue[button][floor] == 1 {
				return true
			}
		}
	}
	return false
}

//HasOrderBelow returns whether an elev has a local order below last known floor
func HasOrderBelow(elev ElevData) bool {
	for floor := 0; floor < elev.Floor; floor++ {
		for button := 0; button < config.NumButtonTypes; button++ {
			if elev.LocalQueue[button][floor] == 1 {
				return true
			}
		}
	}
	return false
}
func hasOrderAtCurrentFloor(elev ElevData) bool {
	for button := 0; button < config.NumButtonTypes; button++ {
		if elev.LocalQueue[button][elev.Floor] == 1 {
			return true
		}
	}
	return false
}

func hasHallUpOrderAtCurrentFloor(elev ElevData) bool {
	return elev.LocalQueue[elevio.BT_HallUp][elev.Floor] == 1
}

func hasCabOrderAtCurrentFloor(elev ElevData) bool {
	fmt.Println(elev)
	return elev.LocalQueue[elevio.BT_Cab][elev.Floor] == 1
}

func hasHallDownOrderAtCurrentFloor(elev ElevData) bool {
	return elev.LocalQueue[elevio.BT_HallDown][elev.Floor] == 1
}

func addOrderToQueue(queue [][]int, newOrder elevio.ButtonEvent) [][]int {
	queue[newOrder.Button][newOrder.Floor] = 1
	return queue
}

// ShoudClearOrder return true if an order
func shouldClearOrder(elev ElevData, order elevio.ButtonEvent) bool {
	switch order.Button {
	case elevio.BT_Cab:
		if hasCabOrderAtCurrentFloor(elev) {
			return true
		}
	case elevio.BT_HallUp:
		if hasHallUpOrderAtCurrentFloor(elev) {
			if elev.HeadingDir == HeadingUp {
				return true
			}
			if !hasHallDownOrderAtCurrentFloor(elev) &&
				!HasOrderBelow(elev) {
				return true
			}
		}

	case elevio.BT_HallDown:
		if hasHallDownOrderAtCurrentFloor(elev) {
			if elev.HeadingDir == HeadingDown {
				return true
			}
			if !hasHallUpOrderAtCurrentFloor(elev) &&
				!hasOrderAbove(elev) {
				return true
			}
		}
	}
	return false
}

func turnOffLightsOfClearedOrders(elev ElevData) {
	//Clearing cab order
	elevio.SetButtonLamp(elevio.BT_Cab, elev.Floor, false)

	//Clearing prioritized hall order
	switch elev.HeadingDir {
	case HeadingUp:
		elevio.SetButtonLamp(elevio.BT_HallUp, elev.Floor, false)
	case HeadingDown:
		elevio.SetButtonLamp(elevio.BT_HallDown, elev.Floor, false)
	default:
	}
}

func getMotorDirection(headingDirection HeadingDirection) elevio.MotorDirection {
	if headingDirection == HeadingUp {
		return elevio.MD_Up
	}
	return elevio.MD_Down
}
