import psutil, time

sep = ','
header_printed = False

while True:
    infos = psutil.cpu_times_percent(interval=1, percpu=True)
    if not header_printed:
        header_printed = True
        header = [
            'timestamp',
            *[
                f'{t}.{i}'
                for i in range(1, len(infos)+1)
                for t in ('user', 'system')
            ]
        ]
        print(sep.join(header))
    row = [
        int(time.time()),
        *[
            t
            for info in infos
            for t in (info.user, info.system)
        ]
    ]
    row = map(str, row)
    print(sep.join(row))

