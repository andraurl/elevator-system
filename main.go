package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"time"

	"./network/bcast"
	"./network/peers"

	"./config"
	dist "./distribution"
	sync "./synchronization"

	"./elevio"
	"./esm"
	"./network/localip"
)

func main() {

	var myID string
	var simPort int
	flag.StringVar(&myID, "myID", "", "myID of this peer")
	flag.IntVar(&simPort, "simPort", 15657, "Simulator connection port")
	flag.Parse()
	if myID == "" {
		localIP, err := localip.LocalIP()
		if err != nil {
			fmt.Println(err)
			localIP = "DISCONNECTED"
		}
		// myID = fmt.Sprintf("peer-%s-%d", localIP, os.Getpid())
		myID = fmt.Sprintf("peer-%s", localIP)
	}

	// hardware -> distribution
	buttonPressed := make(chan elevio.ButtonEvent)
	// hardware -> esm
	arrivedAtFloor := make(chan int)

	// distribution -> esm
	turnOnLight := make(chan elevio.ButtonEvent)
	turnOffLight := make(chan elevio.ButtonEvent)
	newOrder := make(chan elevio.ButtonEvent)
	watchDogTimeOut := make(chan bool)

	// distribution -> synchronization
	hallOrder := make(chan elevio.ButtonEvent)

	// esm -> synchronization
	localElevData := make(chan esm.ElevData)
	completedOrder := make(chan elevio.ButtonEvent)

	// synchronization -> distribution
	clearedOrderStatusOrder := make(chan elevio.ButtonEvent)
	syncedElevData := make(chan []esm.ElevData)

	// synchronization -> network
	outgoingMsg := make(chan esm.ElevData)
	transmitEnable := make(chan bool)

	// network to synchronization
	incomingMsg := make(chan esm.ElevData)
	peerUpdateCh := make(chan peers.PeerUpdate)

	esmChannels := esm.Channels{
		NewOrder:        newOrder,
		CompletedOrder:  completedOrder,
		ArrivedAtFloor:  arrivedAtFloor,
		WatchDogTimeOut: watchDogTimeOut,
		TurnOnLight:     turnOnLight,
		TurnOffLight:    turnOffLight,
		LocalElevData:   localElevData,
	}

	distributionChannels := dist.Channels{
		ButtonPressed:           buttonPressed,
		NewOrder:                newOrder,
		SyncedElevData:          syncedElevData,
		WatchDogTimeOut:         watchDogTimeOut,
		TurnOnLight:             turnOnLight,
		TurnOffLight:            turnOffLight,
		ClearedOrderStatusOrder: clearedOrderStatusOrder,
		HallOrder:               hallOrder,
	}

	syncChannels := sync.Channels{
		IncomingMsg:             incomingMsg,
		OutgoingMsg:             outgoingMsg,
		SyncedElevData:          syncedElevData,
		LocalElevData:           localElevData,
		TransmitEnable:          transmitEnable,
		PeerUpdateCh:            peerUpdateCh,
		CompletedOrder:          completedOrder,
		HallOrder:               hallOrder,
		ClearedOrderStatusOrder: clearedOrderStatusOrder,
	}

	// Connect to server
	connectionPort := fmt.Sprintf("localhost:%d", simPort)

	// Initiate elevator
	initFloor := elevio.Init(connectionPort, config.NumFloors)

	// Start network communication
	go bcast.Receiver(20017, incomingMsg)
	go bcast.Transmitter(20017, outgoingMsg)
	go peers.Receiver(20018, peerUpdateCh)
	go peers.Transmitter(20018, myID, transmitEnable)

	// Start elevator polling
	go elevio.PollButtons(buttonPressed)
	go elevio.PollFloorSensor(arrivedAtFloor)
	go killSwitch()

	// Module
	go dist.Distribute(distributionChannels, myID)
	go esm.ESM(esmChannels, initFloor)
	go sync.Synchronize(syncChannels, myID)

	select {}
}

func killSwitch() {
	// killSwitch turns the motor off if the program is killed with CTRL+C.
	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt)
	<-c
	elevio.SetMotorDirection(elevio.MD_Stop)
	fmt.Println("\x1b[31;1m", "User terminated program.", "\x1b[0m")
	for i := 0; i < 10; i++ {
		elevio.SetMotorDirection(elevio.MD_Stop)
		if i%2 == 0 {
			elevio.SetStopLamp(true)
		} else {
			elevio.SetStopLamp(false)
		}
		time.Sleep(200 * time.Millisecond)
	}
	elevio.SetMotorDirection(elevio.MD_Stop)
	os.Exit(1)
}
