// Package api defines the messages Snow's client and server exchange and how
// they are framed on the wire. It is the only public package: other clients,
// such as an editor extension, use it to talk to a Snow server.
//
// Every frame is a 4-byte big-endian payload length, a 1-byte message type,
// and a JSON payload. Message types never change number once released.
package api

// ProtocolVersion changes when a client and server of different versions can
// no longer understand each other.
const ProtocolVersion = 1
