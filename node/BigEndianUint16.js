export default class BigEndianUint16 {
    constructor(num) {
        this.lo = (num >> 8) & 0xFF;
        this.hi = num & 0xFF;
    }
}