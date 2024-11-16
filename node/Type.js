export default class Type {
    static pub = 1;
    static sub = 2;
    static unsub = 4;

    static str(t) {
        if (t == Type.pub) {
            return "pub";
        }
        if (t == Type.sub) {
            return "sub";
        }
        if (t == Type.unsub) {
            return "unsub";
        }
        return `<invalid ${t}>`;
    }
};