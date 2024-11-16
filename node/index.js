import * as net from 'net';
import MessageFSM from './MessageFSM.js';
import PubSubServer from './PubSubServer.js';
import BigEndianUint16 from './BigEndianUint16.js';

let netsvOpts = {
    noDelay: true,
};
let netsv = net.createServer(netsvOpts);
let sv = new PubSubServer();

netsv.on('error', err => {
    console.log('server error', err);
});

netsv.on('connection', sock => {
    sock.on('end',     () => { sv.disconnect(sock); });
    sock.on('timeout', () => { sock.end(); });
    sock.on('error',   () => { sock.end(); }); // err ignored

    function resetTimeout() {
        sock.setTimeout(60_000);
    }
    resetTimeout();

    let fsm = new MessageFSM();
    fsm.onPing(() => {
        let z = new BigEndianUint16(0);
        sock.write(Buffer.of(z.lo, z.hi));
    });
    fsm.onSub    (topic        => { sv.subscribe(topic, sock); });
    fsm.onUnsub  (topic        => { sv.unsubscribe(topic, sock); });
    fsm.onPub    ((topic, buf) => { sv.publish(topic, buf); });
    fsm.onUnknown(()           => sock.end());

    sock.on('data', data => {
        resetTimeout();
        for (let byte of data) {
            fsm.handle(byte);
        }
    });
});


let host =        process.argv[2]  || 'localhost'
let port = Number(process.argv[3]) || 0 // ephemeral

let listenOpts = {
    host,
    port,
};
netsv.listen(listenOpts, () => {
    let {address, port} = netsv.address();
    console.log(`listening on ${address}:${port}`);
});