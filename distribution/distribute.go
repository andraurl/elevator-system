package distribution

import (
	"math"
	"strings"

	"../config"

	"../elevio"
	"../esm"
)

// Channels are channels used by Distribution to communiate with other modules.
type Channels struct {
	ButtonPressed           chan elevio.ButtonEvent
	NewOrder                chan elevio.ButtonEvent
	SyncedOrderStatus       chan [][]int
	SyncedElevData          chan []esm.ElevData
	WatchDogTimeOut         chan bool
	TurnOnLight             chan elevio.ButtonEvent
	TurnOffLight            chan elevio.ButtonEvent
	ClearedOrderStatusOrder chan elevio.ButtonEvent
	HallOrder               chan elevio.ButtonEvent
}

// shouldTakeOrder checks if the current elevator should take the order by comparing costs
func shouldTakeOrder(elevData []esm.ElevData, order elevio.ButtonEvent) bool {
	if isAlone(elevData) && elevData[0].State != esm.Undefined {
		return true
	}
	bestElevID := ""
	bestElevCost := math.MaxInt64
	myID := elevData[0].ID

	for _, elev := range elevData {
		if elev.Online && elev.State != esm.Undefined {
			cost := GetCost(elev, order)
			println("Cost: ", cost)
			println("bestElevCost:", bestElevCost)
			println("myID: ", myID)
			println("bestElevID: ", bestElevID)
			if cost < bestElevCost || (cost == bestElevCost && strings.Compare(elev.ID, bestElevID) == 1) {

				bestElevID = elev.ID
				bestElevCost = cost
			}
		}
	}
	return bestElevID == myID
}

//Distribute func distributes
func Distribute(channels Channels, myID string) {

	elevData := make([]esm.ElevData, config.MaxNumElevators)

	confirmedOrder := make(chan elevio.ButtonEvent)

	distributedOrders := make([][]int, config.NumButtonTypes)
	for i := 0; i < config.NumButtonTypes; i++ {
		distributedOrders[i] = make([]int, config.NumFloors)
	}

	for {
		select {

		case buttonPressed := <-channels.ButtonPressed:
			println("Received buttonpress")
			if buttonPressed.Button == elevio.BT_Cab {
				println("Buttonpress is cab order. Adding to local queue")
				elevio.SetButtonLamp(buttonPressed.Button, buttonPressed.Floor, true)
				go func() { channels.NewOrder <- buttonPressed }()
			} else {
				println("Buttonpress is hall order. Sending to sync")
				go func() { channels.HallOrder <- buttonPressed }()
			}

		// On new elevData check for new orders.
		// Check for completed orders: Send slukk lys.
		case syncedElevData := <-channels.SyncedElevData:
			esm.DeepCopy(&elevData, &syncedElevData)

			println("Received elevData from Synchronization: ")
			for elevNr := 0; elevNr < config.MaxNumElevators; elevNr++ {
				//fmt.Println(elevData[elevNr])
			}
			// Checks whether an order is syncronized over all online elevators.
			// If syncronized and not already distributed then distribute it.
			for buttonNr := 0; buttonNr < config.NumButtonTypes; buttonNr++ {
				for floorNr := 0; floorNr < config.NumFloors; floorNr++ {
					syncOrder := true
					if isAlone(elevData) {
						syncOrder = elevData[0].OrderStatus[buttonNr][floorNr] == 1
					} else {
						for elevNr := 0; elevNr < config.MaxNumElevators; elevNr++ {
							if elevData[elevNr].Online && elevData[elevNr].OrderStatus[buttonNr][floorNr] != 1 {
								syncOrder = false
							}
						}
					}

					if syncOrder {
						println("Synchronized order at floor: ", floorNr, " button: ", buttonNr)
						if distributedOrders[buttonNr][floorNr] == 0 {
							distributedOrders[buttonNr][floorNr] = 1
							order := elevio.MakeButtonEvent(buttonNr, floorNr)
							go func() { channels.TurnOnLight <- order }()
							go func() { confirmedOrder <- order }()

						} else {
							println("Order is already distributed")
						}
					}
				}
			}

		case order := <-channels.ClearedOrderStatusOrder:
			go func() { channels.TurnOffLight <- order }()
			distributedOrders[int(order.Button)][order.Floor] = 0

		case order := <-confirmedOrder:
			println("Confirmed order: Floor:", order.Floor, " Button:", order.Button)
			if shouldTakeOrder(elevData, order) {
				println("Distributed order to thisElev: ", myID)
				go func() { channels.NewOrder <- order }()
			}

		// On WatchDogTimeOut: Redistribute all distributedOrders.
		case <-channels.WatchDogTimeOut:
			println("Dist: WatchDogTimer timeout")
			for buttonNr := 0; buttonNr < config.NumButtonTypes; buttonNr++ {
				for floorNr := 0; floorNr < config.NumFloors; floorNr++ {
					if distributedOrders[buttonNr][floorNr] == 1 {
						order := elevio.MakeButtonEvent(buttonNr, floorNr)
						go func() { confirmedOrder <- order }()
					}
				}
			}
		}
	}
}

func isAlone(elevData []esm.ElevData) bool {

	for i := range elevData {
		if elevData[0].ID != elevData[i].ID && elevData[i].Online {
			return false
		}
	}
	return true
}
