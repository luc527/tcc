import {parentPort} from 'node:worker_threads';

parentPort.on('message', msg => {
    let a_ = 'a'.charCodeAt(0);
    let z_ = 'z'.charCodeAt(0);
    let A_ = 'A'.charCodeAt(0);
    let Z_ = 'Z'.charCodeAt(0);

    let {topic, payload} = msg;
    let b = Buffer.alloc(payload.length);
    let bi = 0;
    for (let pi = 0; pi < payload.length; pi++) {
        let c = payload[pi];
        if (c >= a_ && c <= z_) {
            c = ((c - a_ + 13) % 26) + a_;
            b[bi++] = c;
        }
        else if (c >= A_ && c <= Z_) {
            c = ((c - A_ + 13) % 26) + A_;
            b[bi++] = c;
        }
    }
    b = b.subarray(0, bi);
    b.sort();
    parentPort.postMessage({
        topic,
        payload: b,
    });
});