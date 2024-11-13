import re
from collections import defaultdict

dir = './data_go_1211_2143'
# dir = './data_node_1211_2225'

actions = defaultdict(list)

observations = defaultdict(list)

with open(f'{dir}/out.txt') as f:
    while (line := f.readline()):
        line = line.rstrip()
        match = re.match(r'(\w+): (\d+) (.*)', line)
        
        kind = match.group(1)
        timestamp = int(match.group(2))
        rest = match.group(3)

        if kind == "dbg":
            continue

        if kind == "in":
            if rest.startswith('subs'):
                match = re.match(r'subs n=(\d+) topic=(\d+)', rest)
                n = int(match.group(1))
                topic = int(match.group(2))
                actions[timestamp].append({
                    'action': 'subs',
                    'n': n,
                    'topic': topic,
                })
            elif rest.startswith('pubs'):
                match = re.match(r'pubs n=(\d+) topic=(\d+) intervalMs=(\d+) payload=(\S+)', rest)
                n = int(match.group(1))
                topic = int(match.group(2))
                interval_ms = int(match.group(2))
                payload = match.group(3)
                actions[timestamp].append({
                    'action': 'pubs',
                    'n': n,
                    'topic': topic,
                    'interval_ms': interval_ms,
                    'payload': payload
                })
            elif rest.startswith('monitors'):
                match = re.match(r'monitors n=(\d+) topic=(\d+) intervalMs=(\d+)', rest)
                n = int(match.group(1))
                topic = int(match.group(2))
                interval_ms = int(match.group(3))
                actions[timestamp].append({
                    'action': 'monitors',
                    'n': n,
                    'topic': topic,
                    'interval_ms': interval_ms,
                })

        if kind == "out":
            match = re.match(r'observation topic=(\d+) delayMs=(\d+)', rest)
            topic = int(match.group(1))
            delay_ms = int(match.group(2))

            observations[timestamp].append({
                'topic': topic,
                'delay_ms': delay_ms,
            })

cpu_measurements = defaultdict(dict)

with open(f'{dir}/cpu.csv') as f:
    f.readline()
    while (line := f.readline()):
        line = line.rstrip()
        [timestamp, *rest] = line.split(',')
        timestamp = int(timestamp)

        for cpu in range(0, int(len(rest)/2)):
            user   = float(rest[2*cpu])
            system = float(rest[2*cpu+1])

            cpu_measurements[cpu][timestamp] = {
                'user': user,
                'system': system,
            }

# for now interested only in uss

mem_measurements = {}

with open(f'{dir}/mem.csv') as f:
    f.readline()
    while (line := f.readline()):
        line = line.rstrip()
        [timestamp, _, _, uss, *_] = line.split(',')
        timestamp = int(timestamp)
        uss = int(uss)

        mem_measurements[timestamp] = uss

# reindex

beginning = min(actions.keys())

actions      = {t - beginning: v for t, v in actions.items()}
observations = {t - beginning: v for t, v in observations.items()}

cpu_measurements = {cpu:
                    {t - beginning: v for t, v in measurements.items() if t >= beginning}
                    for cpu, measurements in cpu_measurements.items()}

mem_measurements = {t - beginning: v for t, v in mem_measurements.items() if t >= beginning}

from pprint import pprint
pprint(cpu_measurements)
