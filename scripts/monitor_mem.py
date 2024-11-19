import sys, psutil, time

if len(sys.argv) == 1:
    print('pid?')
    exit

pid = int(sys.argv[1])
p = psutil.Process(pid)

sep = ','
header = [
    'timestamp',
    'uss',
]
print(sep.join(header))

while True:
    time.sleep(1)
    info = p.memory_full_info()
    timestamp = int(time.time())
    row = map(str, [
        timestamp,
        info.uss,
    ])
    print(sep.join(row))


