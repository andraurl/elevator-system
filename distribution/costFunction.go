package distribution

import (
	"../elevio"
	"../esm"
)

func isMovingTowards(elevData esm.ElevData, order elevio.ButtonEvent) bool {

	if elevData.Floor == order.Floor &&
		elevData.State == esm.Moving {
		return false
	}

	switch elevData.HeadingDir {

	case esm.HeadingUp:
		switch order.Button {
		case elevio.BT_Cab:
			return elevData.Floor <= order.Floor
		case elevio.BT_HallUp:
			return elevData.Floor <= order.Floor
		case elevio.BT_HallDown:
			return false
		}

	case esm.HeadingDown:
		switch order.Button {
		case elevio.BT_Cab:
			return order.Floor <= elevData.Floor
		case elevio.BT_HallUp:
			return false
		case elevio.BT_HallDown:
			return order.Floor <= elevData.Floor
		}
	}
	return false
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// GetCost a ish cost for elevator. More simple implementation that won't crash
func GetCost(elev esm.ElevData, order elevio.ButtonEvent) int {

	cost := 0
	switch elev.State {

	case esm.Idle:
		return abs(elev.Floor - order.Floor)

	case esm.DoorOpen:
		return 1 + abs(elev.Floor-order.Floor)

	case esm.Moving:
		if !isMovingTowards(elev, order) {
			cost += 5
		} else {
			cost += abs(elev.Floor-order.Floor) * 2
		}
	}
	return cost
}
