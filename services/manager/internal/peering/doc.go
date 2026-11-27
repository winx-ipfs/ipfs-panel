// Package peering keeps a set of peers connected.
//
// The saved list is Kubo's Peering.Peers. The running list is the swarm
// peering service. Add and remove write both, so a restart is not required
// when both calls succeed. The browser never receives a Kubo RPC address.
package peering
