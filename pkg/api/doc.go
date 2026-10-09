// Package api defines the messages Snow's client and server exchange and how
// they are framed on the wire. It is the only public package: other clients,
// such as an editor extension, use it to talk to a Snow server.
//
// Every frame is a 4-byte big-endian payload length, a 1-byte message type,
// and a JSON payload.
//
// Compatibility rules:
//
//   - Adding a field or a message type keeps ProtocolVersion. Receivers ignore
//     unknown fields, and ReadFrame skips unknown message types.
//   - Removing or retyping a field, or changing what a message means, bumps
//     ProtocolVersion.
//   - The frame header, Hello, Welcome, and Error never change shape, so
//     peers of any version can always detect a mismatch.
//   - Message types never change number once released.
//   - Arrays are never null; an empty array is [].
//   - Pane and window IDs start at 1; 0 means none.
package api

// ProtocolVersion changes when a client and server of different versions can
// no longer understand each other.
const ProtocolVersion = 1
