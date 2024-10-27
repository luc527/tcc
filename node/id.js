let _wm = new WeakMap();
let _next = 1;

export function getid(obj) {
    if (_wm.has(obj)) {
        return _wm.get(obj);
    } else {
        let id = _next++;
        _wm.set(obj, id);
        return id;
    }
}