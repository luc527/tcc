import BigEndianUint16 from "./BigEndianUint16.js";
import Type from "./Type.js";
import {getid} from './id.js';

import {Worker} from 'node:worker_threads';

const nworkers = 8;
let prefixbuf = Buffer.from('!sumall ');

export default class PubSubServer {
    constructor() {
        this.subscribers = new Map();
        this.topics = new Map();
        
        this.workers = [];
        for (let i = 0; i < nworkers; i++) {
            let worker = new Worker('./sumall.js');
            worker.on('online', () => { console.log(`worker ${i} is online`); });
            worker.on('error', err => { console.log(`worker ${i} error: ${err}`); });
            worker.on('exit', () => { console.log(`worker ${i} exited?!`); });
            worker.on('messageerror', err => { console.log(`worker ${i} messageerror: ${err}`); });
            worker.on('message', msg => {
                let {topic, payload} = msg;
                this.publish(topic, payload);
            });
            this.workers[i] = worker;
        }
        this.nextworker = 0;
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
        let topics = this.topics.get(sock);
        if (!topics) {
            topics = new Set();
            this.topics.set(sock, topics);
        }
        if (!topics.has(topic)) {
            topics.add(topic);
    
            let subs = this.subscribers.get(topic);
            if (!subs) {
                subs = new Set();
                this.subscribers.set(topic, subs);
            }
            subs.add(sock);
        }

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
        if (0 == Buffer.compare(prefixbuf, payloadbuf.slice(0, prefixbuf.length))) {
            let worker = this.workers[this.nextworker];
            worker.postMessage({
                topic,
                payload: payloadbuf.subarray(prefixbuf.length),
            });
            this.nextworker = (this.nextworker + 1) % this.workers.length;
            return;
        }

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