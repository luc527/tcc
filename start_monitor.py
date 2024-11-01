import os, sys, subprocess, psutil, time

# e.g. $ python start_monitor.py ./tccgo server 127.0.0.1:
server_args = sys.argv[1:]

sp = subprocess.Popen(server_args, stdout=subprocess.PIPE)
print(f'pid: {sp.pid}')
print(f'out: {sp.stdout.readline().decode().rstrip()}')

tgid = sp.pid
pcache = {} # to avoid creating new psutil.Process instances every loop iteration

while True:
    time.sleep(2)

    if (code := sp.poll()):
        print(f'exit ({code})')
        break

    prev_tids = set(pcache.keys())
    curr_tids = set(map(int, os.listdir(f'/proc/{tgid}/task')))

    pcache = {tid: pcache[tid] if tid in pcache else psutil.Process(tid)
              for tid in curr_tids}

    vet_tids = curr_tids & prev_tids  # "veteran"
    new_tids = curr_tids - prev_tids

    vet_ps = {pcache[tid] for tid in vet_tids}
    new_ps = {pcache[tid] for tid in new_tids}
    all_ps = pcache.values()

    for p in new_ps:
        p.cpu_percent(0)

    # TODO: find out which memory metric to take the percent of
    # (full info in p.memory_info())

    # TODO: sort cpu nums

    percs = {p.cpu_num(): p.cpu_percent(0) for p in vet_ps}
    mems  = {p.cpu_num(): p.memory_percent() for p in all_ps}

    print('CPU:', ', '.join(f'{p:>5.2f}%' for p in percs))
    print('MEM:', ', '.join(f'{m:>5.2f}%' for m in mems))
    print()


