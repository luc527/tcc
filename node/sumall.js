import {parentPort} from 'node:worker_threads';

parentPort.on('message', msg => {
    let {topic, payload} = msg;
    let buf = Buffer.alloc(payload.length);
    let a_ = 'a'.charCodeAt(0);
    for (let i = 0; i < payload.length; i++) {
        let r = 0;
        for (let j = 0; j < payload.length; j++) {
            for (let k = 0; k < payload.length; k++) {
                r = (r + payload[i] * j + payload[k]) & 0xFF;
            }
        }
        buf[i] = r % 26 + a_;
    }
    parentPort.postMessage({
        topic,
        payload: buf,
    });
});