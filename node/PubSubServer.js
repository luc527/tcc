import BigEndianUint16 from "./BigEndianUint16.js";
import Type from "./Type.js";
import {getid} from './id.js';

export default class PubSubServer {
    constructor() {
        this.subscribers = new Map();
        this.topics = new Map();
    }
    
    dbg() {
        console.log('\npubsub> SUBSCRIBERS');
        console.log(
            Array.from(this.subscribers.entries()).map(([topic, subs]) =>
                `{topic: ${topic}, subs: [${Array.from(subs).map(x => getid(x).toString()).join(', ')}]}`
            ).join('; ')
        );
        console.log('pubsub> TOPICS');
        console.log(
            Array.from(this.topics.entries()).map(([sub, topics]) => 
                `{sub: ${getid(sub)}, topics: [${Array.from(topics).map(t => t.toString()).join(', ')}]}`
            ).join('; ')
        );
    }

    subscribe(topic, sock) {
        let subs = this.subscribers.get(topic);
        if (!subs) {
            subs = new Set();
            this.subscribers.set(topic, subs);
        }
        subs.add(sock);

        let topics = this.topics.get(sock);
        if (!topics) {
            topics = new Set();
            this.topics.set(sock, topics);
        }
        topics.add(topic);

        let size16 = new BigEndianUint16(3); // type + topic
        let topic16 = new BigEndianUint16(topic);
        let msgbuf = Buffer.of(size16.lo, size16.hi, Type.sub, topic16.lo, topic16.hi);
        sock.write(msgbuf);

        // console.log(`pubsub> sub ${getid(sub)} subscribed to topic ${topic}`);
        // this.dbg();
    }

    unsubscribe(topic, sock) {
        let subs = this.subscribers.get(topic);
        if (subs) {
            subs.delete(sock);
            if (subs.size == 0) {
                this.subscribers.delete(topic);
            }
        }

        let topics = this.topics.get(sock);
        if (topics) {
            topics.delete(topic);
            if (topics.size == 0) {
                this.topics.delete(sock);
            }
        }

        let size16 = new BigEndianUint16(3); // type + topic
        let topic16 = new BigEndianUint16(topic);
        let msgbuf = Buffer.of(size16.lo, size16.hi, Type.unsub, topic16.lo, topic16.hi);
        sock.write(msgbuf);

        // console.log(`pubsub> sub ${getid(sub)} unsubscribed from topic ${topic}`);
        // this.dbg();
    }

    disconnect(sub) {
        let topics = this.topics.get(sub);
        if (!topics) {
            return;
        }
        for (let topic of topics) {
            let subs = this.subscribers.get(topic);
            subs.delete(sub);
            if (subs.size == 0) {
                this.subscribers.delete(topic);
            }
        }
        this.topics.delete(sub);

        // console.log(`pubsub> sub ${getid(sub)} disconnected`);
        // this.dbg();
    }

    publish(topic, payloadbuf) {
        let subs = this.subscribers.get(topic);
        if (!subs) {
            return;
        }
        let size = 1 + 2 + payloadbuf.length;
        let msgbuf = Buffer.alloc(2 + size);
        let i = 0;

        let size16 = new BigEndianUint16(size);
        msgbuf[i++] = size16.lo;
        msgbuf[i++] = size16.hi;

        msgbuf[i++] = Type.pub;

        let topic16 = new BigEndianUint16(topic);
        msgbuf[i++] = topic16.lo;
        msgbuf[i++] = topic16.hi;

        for (let byte of payloadbuf) {
            msgbuf[i++] = byte;
        }
        
        for (let sock of subs) {
            sock.write(msgbuf);
        }

        // console.log(`pubsub> publishing message "${payloadbuf.toString('utf8')}" to topic ${topic}`);
    }
}