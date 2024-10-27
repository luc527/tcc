import Type from './Type.js';

// little-endian
class Uint16FSM {
    constructor() {
        this.ok = false;
        this.value = 0;
        this.read = 0;
    }

    handle(byte) {
        switch (this.read) {
        case 0:
            this.value = byte;
            this.read = 1;
            this.ok = false;
            break;
        case 1:
            this.value = (byte << 8) | this.value;
            this.read = 0;
            this.ok = true;
            break;
        }
    }
}

class PayloadFSM {
    constructor() {
        this.reset(0);
    }

    reset(toRead) {
        this.buffer = Buffer.alloc(toRead);
        this.read = 0;
        this.ok = false;
    }

    // XXX: possible bottleneck? this runs for *each byte* of the payload
    handle(byte) {
        if (this.read < this.buffer.length) {
            this.buffer[this.read++] = byte;
            if (this.read == this.buffer.length) {
                this.ok = true;
            }
        }
    }
}

export default class MessageFSM {
    constructor() {
        this.uint = new Uint16FSM();
        this.payload = new PayloadFSM();
        this.stateFn = this.start;
    }

    onUnknown(cb) { this.unknownCb = cb; }
    onPing(cb)    { this.pingCb = cb; }
    onPub(cb)     { this.pubCb = cb; }
    onSub(cb)     { this.subCb = cb; }

    handle(byte) {
        this.stateFn(byte);
    }

    start(byte) {
        switch (byte) {
        case Type.ping:
            this.pingCb && this.pingCb();
            break;
        case Type.sub:
            this.type = Type.sub;
            this.stateFn = this.subTopic;
            break;
        case Type.pub:
            this.type = Type.pub;
            this.stateFn = this.pubTopic;
            break;
        default:
            this.unknownCb && this.unknownCb(byte);
            break;
        }
    }

    subTopic(byte) {
        let uint = this.uint;
        uint.handle(byte);
        if (uint.ok) {
            this.topic = uint.value;
            this.stateFn = this.subBoolean;
        }
    }

    subBoolean(byte) {
        let b = byte != 0;
        this.subCb && this.subCb(this.topic, b)
        this.stateFn = this.start;
    }

    pubTopic(byte) {
        let uint = this.uint;
        uint.handle(byte);
        if (uint.ok) {
            this.topic = uint.value;
            this.stateFn = this.pubLength;
        }
    }

    pubLength(byte) {
        let uint = this.uint;
        uint.handle(byte);
        if (uint.ok) {
            let length = uint.value;
            this.payload.reset(length);
            this.stateFn = this.pubPayload;
        }
    }

    pubPayload(byte) {
        let payload = this.payload;
        payload.handle(byte);
        if (payload.ok) {
            let buffer = payload.buffer;
            this.pubCb && this.pubCb(this.topic, buffer);
            this.stateFn = this.start;
        }
    }
}