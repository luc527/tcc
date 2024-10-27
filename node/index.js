import * as net from 'net';
import MessageFSM from './MessageFSM.js';
import Type from './Type.js';
import PubSubServer from './PubSubServer.js';

let netsvOpts = {
    noDelay: true,
};
let netsv = net.createServer(netsvOpts);

let pubsub = new PubSubServer();

netsv.on('error', err => {
    console.log('server error', err);
});

netsv.on('connection', sock => {
    sock.on('end', () => {
        pubsub.disconnect(sock);
    });

    sock.on('timeout', () => {
        console.log('sock timed out');
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
            pubsub.subscribe(topic, sock);
        } else {
            pubsub.unsubscribe(topic, sock);
        }
    });
    fsm.onPub((topic, buf) => {
        pubsub.publish(topic, buf);
    });

    sock.on('data', data => {
        resetTimeout();
        for (let byte of data) {
            fsm.handle(byte);
        }
    });
});

let listenOpts = {
    host: 'localhost',
    port: 0, // ephemeral
};
netsv.listen(listenOpts, () => {
    let {port} = netsv.address();
    console.log(`listening on port ${port}`);
});