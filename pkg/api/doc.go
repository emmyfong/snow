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
//     ProtocolVersion. A new message that changes what an old one, or a
//     closed connection, means also bumps it. Version 2 added the
//     session-ended Error, so a close without it now means a lost connection.
//     Version 3 added Errors that leave the session running (bad-command,
//     command-failed), which older clients treated as fatal. Version 6
//     sends SessionList to attached clients too; before, only clients that
//     had not attached got it.
//   - The frame header, Hello, Welcome, Error, and Shutdown never change
//     shape, so peers of any version can always detect a mismatch, and a
//     newer client can always ask an older server to stop.
//   - Message types never change number once released.
//   - Arrays are never null; an empty array is [].
//   - Pane IDs start at 1; 0 means none. Window indexes start at 0, as in
//     tmux.
package api

// ProtocolVersion changes when a client and server of different versions can
// no longer understand each other.
const ProtocolVersion = 6
