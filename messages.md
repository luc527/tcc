## Types

| Direction        | Abbreviation | Arguments | Purpose |
| ---------------- | ------------ | --------- | ------- |
| Server to client | `ping`       | _{}_ | Prompts the client to send a `pong` back.
| Client to server | `pong`       | _{}_ | Acknowledges a `ping` sent by the server.
| Client to server | `talk`       | _{room, text}_ | Sends a message containing _text_ to the _room_.
| Server to client | `hear`       | _{room, text, name}_ | Informs the client that the user _name_ sent a message containing _text_ to the _room_.
| Client to server | `join`       | _{room, name}_ | Join the _room_ with the given display _name_.
| Server to client | `jned`       | _{room, name}_ | Informs the client that someone joined the _room_ with the given display _name_.
| Client to server | `exit`       | _{room}_  | Exit the _room_.
| Server to client | `exed`       | _{room, name}_ | Informs the client that the user with the given display _name_ has exited the _room_.
| Client to server | `lsro`       | _{}_ | Request a list of the rooms the client has joined, and with which name.
| Server to client | `rols`       | _{text}_ | Responds with a room list in CSV (`room,name`).
| Server to client | `prob`       | _{error code}_ | Informs the client that some problem has happened when processing the previous message.

The possible errors returned with a `prob` message are the following:

| Abbreviation | Caused by        | Description |
| ------------ | ---------------- | - |
| `ejoined`    | `join`           | The user tried to join a room they're currently a member of already.
| `ebadname`   | `join`           | The display name is empty or too long.
| `enameinuse` | `join`           | The given display name is already in use.
| `eroomlimit` | `join`           | The user has reached the limit of how many rooms they can join.
| `eroomfull`  | `join`           | The room has reached the limit of how many members it can have.
| `ebadmes`    | `talk`           | The message is empty or too long.
| `ebadroom`   | `talk` or `exit` | The user is not a member of the given room.
| `etransient` | Any message type | The operation failed because of a transient error, so it will probably succeed if the user just tries again.
| `ebadtype`   | Does not apply   | The server received a message with an invalid type.


## Wire protocol

In this section I describe how each type of message is encoded in binary as a list of fields separated by commas.
Each field can be one of:
- A literal `<byte>`, where `<byte>` is a single byte written as a hexadecimal number;
- An 8-bit unsigned integer `<name>:1`, encoded in a single byte;
- A 16-bit unsigned integer `<name>:2`, encoded in two bytes, little-endian;
- A 32-bit unsigned integer `<name>:4`, encoded in four bytes, little-endian;
- A variable-length UTF-8 string `<name>:^<length-field>`, where `<length-field>` is the name of a previous field that contains the length of the string in bytes as its value.

| Type   | Template |
| ------ | - |
| `ping` | `80` |
| `pong` | `00` |
| `talk` | `01`, `room:4`, `textlen:2`, `text:^textlen` |
| `hear` | `81`, `room:4`, `namelen:1`, `textlen:2`, `name:^namelen`, `text:^textlen` |
| `join` | `02`, `room:4`, `namelen:1`, `name:^namelen` |
| `jned` | `82`, `room:4`, `namelen:1`, `name:^namelen` |
| `exit` | `04`, `room:4` |
| `exed` | `84`, `room:4`, `namelen:1`, `name:^namelen` |
| `lsro` | `08` |
| `rols` | `08`, `textlen:2`, `text:^textlen` |
| `prob` | `90`, `room:4` |

Let's look at a few examples. An user has joined the room `6550` with the display name `superuser` and has sent the message `hello world`.
- The room `6550` is written as `1996` in hexadecimal, so it'll be written as the four bytes `96 16 00 00`.
- The display name `superuser` has length 9, which is just 9 in hexadecimal. Therefore, the `namelen` field will contain the byte `09`.
- The message text `hello world` has length 11, which is B in hexadecimal. Therefore, the `textlen` field will contain the bytes `0B 00`.

Putting it all together, here is how each message would be encoded[^1]:
- `join`: `02 96 16 00 00 09 _s _u _p _e _r _u _s _e _r`
- `jned`: `82 96 16 00 00 09 _s _u _p _e _r _u _s _e _r`
- `talk`: `01 96 16 00 00 0B 00 _h _e _l _l _o _w _o _r _l _d`
- `hear`: `81 96 16 00 00 09 0B 00 _s _u _p _e _r _u _s _e _r _h _e _l _l _o _w _o _r _l _d`

[^1]: Instead of writing the hexadecimal codes for the UTF-8 characters, I decided to just write the caracters directly to make it easier to read.
Which I can do given that none of the characters span more than a single byte.

Finally, here are the binary representations of the `prob` error codes.
Note that the second byte is usually the code of the message type that originated the error.[^2]

| Abbreviation | Binary        |
| ------------ | ------------- |
| `ejoined`    | `01 02 00 00` |
| `ebadname`   | `02 02 00 00` |
| `enameinuse` | `03 02 00 00` |
| `eroomlimit` | `04 02 00 00` |
| `eroomfull`  | `05 02 00 00` |
| `ebadmes`    | `01 01 00 00` |
| `ebadroom`   | `01 05 00 00` |
| `etransient` | `FF xx yy 00`[^3] |
| `ebadtype`   | `60 00 00 00` |

[^2]: There are two exceptions.
The first is `ebadtype`, where the second byte is the code for `prob` negated bitwise.
The second is `ebadroom`, which can originate from either a `talk` or `exit` message, so the second byte is the bitwise OR of their codes.

[^3]: Transient errors can originate from any message.
The second byte `xx` is the code of the message which originated the error.
The third byte `yy` is usually `00` but can contain different values for debugging purposes.
There are no guarantees about what the values are and they will likely be different among the protocol implementations.