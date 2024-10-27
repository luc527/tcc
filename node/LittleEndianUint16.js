export default class LittleEndianUint16 {
    constructor(num) {
        this.lo = num & 0xFF;
        this.hi = (num >> 8) & 0xFF;
    }
}