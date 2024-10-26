import * as net from 'net';
import MessageFSM from './MessageFSM.js';

let serverOpts = {
    noDelay: true,
};
let server = net.createServer(serverOpts);

server.on('error', err => {
    console.log('server error', err);
});

server.on('connection', sock => {
    let fsm = new MessageFSM();
    fsm.onPing(() => {
        console.log('ping!');
    });
    fsm.onSub((topic, b) => {
        console.log(`sub, topic: ${topic}, b: ${b}`);
    });
    fsm.onPub((topic, payload) => {
        console.log(`pub, topic: ${topic}, payload: ${payload}`)
    });
    fsm.onUnknown(() => {
        console.log('unknown message type :(');
    });
    sock.on('data', data => {
        for (let i = 0; i < data.length; i++) {
            fsm.handle(data[i]);
        }
    });
    sock.on('end', () => {
        console.log('sock end');
    });
});

let listenOpts = {
    host: 'localhost',
    port: 0, // ephemeral
};
server.listen(listenOpts, () => {
    let {port} = server.address();
    console.log(`listening on port ${port}`);
});