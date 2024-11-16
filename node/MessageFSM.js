import Type from './Type.js';

// big-endian
class Uint16FSM {
    constructor() {
        this.ok = false;
        this.value = 0;
        this.read = 0;
    }

    handle(byte) {
        switch (this.read) {
        case 0:
            this.value = byte << 8;
            this.read = 1;
            this.ok = false;
            break;
        case 1:
            this.value |= byte;
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

    handle(byte) {
        let d = this.buffer.length - this.read;
        if (d > 0) {
            this.buffer[this.read++] = byte;
            if (d - 1 == 0) {
                this.ok = true;
            }
        }
    }
}

export default class MessageFSM {
    constructor() {
        this.size = 0;
        this.uint = new Uint16FSM();
        this.payload = new PayloadFSM();
        this.stateFn = this.preSize;
    }

    onUnknown(cb) { this.unknownCb = cb; }
    onPing(cb)    { this.pingCb = cb; }
    onPub(cb)     { this.pubCb = cb; }
    onSub(cb)     { this.subCb = cb; }
    onUnsub(cb)   { this.unsubCb = cb; }

    handle(byte) {
        this.stateFn(byte);
    }

    preSize(byte) {
        let uint = this.uint;
        uint.handle(byte);
        if (uint.ok) {
            if (uint.value == 0) {
                this.pingCb && this.pingCb();
                this.stateFn = this.preSize;
            } else {
                this.size = uint.value;
                this.stateFn = this.preType;
            }
        }
    }

    preType(byte) {
        switch (byte) {
        case Type.sub:
            this.stateFn = this.subTopic;
            break;
        case Type.unsub:
            this.stateFn = this.unsubTopic;
            break;
        case Type.pub:
            this.stateFn = this.pubTopic;
            break;
        default:
            this.unknownCb && this.unknownCb(byte);
            this.stateFn = this.preSize;
            break;
        }
    }

    subTopic(byte) {
        let uint = this.uint;
        uint.handle(byte);
        if (uint.ok) {
            let topic = uint.value;
            this.subCb && this.subCb(topic);
            this.stateFn = this.preSize;
        }
    }

    unsubTopic(byte) {
        let uint = this.uint;
        uint.handle(byte);
        if (uint.ok) {
            let topic = uint.value;
            this.unsubCb && this.unsubCb(topic);
            this.stateFn = this.preSize;
        }
    }

    pubTopic(byte) {
        let uint = this.uint;
        uint.handle(byte);
        if (uint.ok) {
            this.topic = uint.value;
            this.payload.reset(this.size - 1 - 2); // - type - topic
            this.stateFn = this.pubPayload;
        }
    }

    pubPayload(byte) {
        let payload = this.payload;
        payload.handle(byte);
        if (payload.ok) {
            let buffer = payload.buffer;
            this.pubCb && this.pubCb(this.topic, buffer);
            this.stateFn = this.preSize;
        }
    }
}