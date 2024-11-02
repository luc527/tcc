import psutil, time, sys, subprocess
from typing import TextIO

# csv field field separator
sep = ';'

args = sys.argv[1:]

if not args:
    print('cpufile?')
    exit()
cpufile, args = args[0], args[1:]

if not args:
    print('memfile?')
    exit()
memfile, args = args[0], args[1:]

if not args:
    print('server args?')
    exit()
subproc = subprocess.Popen(args, stdout=subprocess.PIPE, encoding='utf-8')
proc = psutil.Process(subproc.pid)

print(f'pid: {subproc.pid}')
print(f'out: {subproc.stdout.readline().rstrip()}')

psutil.cpu_times_percent(interval=0, percpu=True)

def write_row(file: TextIO, fields: list):
    file.write(sep.join(map(str, fields)) + '\n')
    file.flush()

with (
    open(cpufile, 'w') as outcpu,
    open(memfile, 'w') as outmem,
):
    write_row(outcpu, ['timestamp', 'cpu', 'user', 'system', 'iowait'])
    write_row(outmem, ['timestamp', 'rss', 'vms', 'uss', 'pss'])

    while True:
        time.sleep(1)
        if subproc.poll():
            break

        timestamp = int(time.time())

        cpu_times = psutil.cpu_times_percent(interval=0, percpu=True)
        for cpu, info in enumerate(cpu_times, start=1):
            write_row(outcpu, [timestamp, cpu, info.user, info.system, info.iowait])

        info = proc.memory_full_info()
        write_row(outmem, [timestamp, info.rss, info.vms, info.uss, info.pss])
