# [TTK4145 Elevator Project](https://www.ntnu.no/studier/emner/TTK4145)

### Description

The elevator project in TTK4145 consisted of controlling `n` elevators such that they would
cooperate and communiate in order to complete given orders. The orders were given using
buttons on elevator control panels. The elevators were to be able to complete orders
on `m` floors.

The main requirements for the elevator were that
  - **No orders are lost**
  We solved this by keeping a local backup of each elevators local queue, as well
  as synchronizing hall orders across all elevators.
  - **Multiple elevators should be more efficient than one**
  We solved this using a Distribution module that calculated which elevator was
  in the best state to complete a given order. This was done for each elevator
  and if found that itself was the best then it would complete the order.
  - **An Individual elevator should behave sensibly and efficiently**
  This was done by completing all orders given if it had no communication with other elevators.
  The orders were completed using a state machine.
  - **The lights should function as expected**
  This was done by illuminating a light when an order was synchronized across
  all online elevators, and turning the light off when an order was completed by
  one elevator.


For more detailed requirements and permitted assumptions check out the
 [`Project Description`](https://github.com/TTK4145/Project#elevator-project).

## Implementation
We implemented our solution using `Google Go`. We chose to control the elevators using
a peer-to-peer solution. This was done by synchronizing each elevators data across
all elevators. Then each elevator would decide only it's own action, based on the
data from the other elevators.
If a hall order order is not completed by another elevator in a given time, then some other
elevator will complete the order.

The elevator was implemented using 5 modules.
  - **esm**
  This module controls the elevator using a state machine in order to complete given orders.
  - **Distribution**
  This module distributes an order to the esm if the order is Syncrhonized and it's own elevator was the best elevator to complete the order. Cab orders would always be distributed.
  - **Synchronization**
  This module synchronizes data between peers, using the Network module, and passing the data
  on to the Distribution module if necessary.
  - **elevio**
  The students of TTK4145 were given access to an elevator driver, for communication with
  the elevator server. This driver can be found at [`driver-go`](https://github.com/TTK4145/driver-go). The elevio module in our implementation is a partly extended version of this driver, where some extra functionality were added.
  - **Network**
  We were also given access to a [`Network Module`](https://github.com/TTK4145/Network-go).
  The Network module is an unedited version of this. This uses *UDP broadcasting* to
  send and receive messages across peers, as well as keeping track of which peers were online.


The system performed well on the acceptance test and completed all orders without mistakes.
It was a fun project and we learned a lot from completing the project. Go was a new language
for us, so we had to learn Go in the process.


## Libraries

As descibed in the **Implementation** section, we were given access to
  - [`elevator driver`](https://github.com/TTK4145/driver-go)
and  
  - [`Network Module`](https://github.com/TTK4145/Network-go)
which is
