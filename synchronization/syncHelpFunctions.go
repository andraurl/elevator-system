package synchronization

import (
	"../config"
	"../esm"
)

func hasPeerChange(elev1 esm.ElevData, elev2 esm.ElevData) bool {
	if elev1.State != elev2.State ||
		elev1.HeadingDir != elev2.HeadingDir ||
		elev1.Floor != elev2.Floor {
		return false
	}
	for i := 0; i < config.NumButtonTypes; i++ {
		for j := 0; j < config.NumFloors; j++ {
			if elev1.LocalQueue[i][j] != elev2.LocalQueue[i][j] ||
				elev1.OrderStatus[i][j] != elev2.OrderStatus[i][j] {
				return false
			}
		}
	}
	return true
}

func hasLocalUpdate(elev1 esm.ElevData, elev2 esm.ElevData) bool {
	if elev1.State != elev2.State ||
		elev1.HeadingDir != elev2.HeadingDir ||
		elev1.Floor != elev2.Floor {
		return false
	}
	for i := 0; i < config.NumButtonTypes; i++ {
		for j := 0; j < config.NumFloors; j++ {
			if elev1.LocalQueue[i][j] != elev2.LocalQueue[i][j] {
				return false
			}
		}
	}
	return true
}

func isAlone(elevData []esm.ElevData) bool {

	for i := range elevData {
		if elevData[0].ID != elevData[i].ID && elevData[i].Online {
			return false
		}
	}
	return true
}
