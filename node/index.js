import * as net from 'net';
import MessageFSM from './MessageFSM.js';
import Type from './Type.js';
import PubSubServer from './PubSubServer.js';

let netsvOpts = {
    noDelay: true,
};
let netsv = net.createServer(netsvOpts);
let sv = new PubSubServer();

netsv.on('error', err => {
    console.log('server error', err);
});

netsv.on('connection', sock => {
    sock.on('end', () => {
        sv.disconnect(sock);
    });

    sock.on('timeout', () => {
        sock.end();
    });

    function resetTimeout() {
        sock.setTimeout(60_000);
    }
    resetTimeout();

    let fsm = new MessageFSM();
    fsm.onPing(() => {
        sock.write(Buffer.of(Type.ping));
    });
    fsm.onSub((topic, b) => {
        if (b) {
            sv.subscribe(topic, sock);
        } else {
            sv.unsubscribe(topic, sock);
        }
    });
    fsm.onPub((topic, buf) => {
        sv.publish(topic, buf);
    });

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
    let {port} = netsv.address();
    console.log(`listening on port ${port}`);
});