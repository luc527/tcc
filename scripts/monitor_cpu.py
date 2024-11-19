import psutil, time

sep = ','
header_printed = False

while True:
    info = psutil.cpu_times_percent(interval=1)
    if not header_printed:
        header_printed = True
        header = [
            'timestamp',
            'user',
            'system',
        ]
        print(sep.join(header))
    row = [
        int(time.time()),
        info.user,
        info.system,
    ]
    row = map(str, row)
    print(sep.join(row))

