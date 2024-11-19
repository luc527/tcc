import sys
import glob
import re
from collections import defaultdict

from common import parse_cpu_data, parse_mem_data

# TODO: will need to remove throughput outliers, group them (avg, min?, max?, percentile?), etc. -- need just one value for each second
# TODO: will need to backfill (gaps in cpu/mem data)

# TODO: subtitles
# TODO: caption
# TODO: normalized y axes for comparing among different languages

def parse_data(lang):
    paths = glob.glob(f'../data/{lang}*')
    for path in paths:
        if '_mem_' in path:
            mempath = path
        elif '_cpu_' in path:
            cpupath = path
        elif '_cli_' in path:
            clipath = path
        else:
            print(f'unknown: {path}')
    trudata, subdata = parse_cli_data(clipath)
    cpudata = parse_cpu_data(cpupath)
    memdata = parse_mem_data(mempath)
    
    return cpudata, memdata, trudata, subdata


def parse_cli_data(path):
    throughput_data = defaultdict(list)
    subs_data       = {}
    with open(path, 'r') as f:
        for line in f:
            line = line.rstrip()
            if line.startswith('pub'):
                pat = r'pub: (\d+) topic=\d+ pub=\d+ msg=\d+ delayMs=(\d+) frame=(\d+) throughputPsec=([\d.]+)'
                m = re.match(pat, line)
                if m:
                    timestamp = int(m.group(1))
                    throughput_data[timestamp].append({
                        'delay_ms': int(m.group(2)),
                        'frame': int(m.group(3)),
                        'throughput_psec': float(m.group(4)),
                    })
            elif line.startswith('dbg'):
                pat = r'dbg: (\d+) subs: (\d+)'
                m = re.match(pat, line)
                if m:
                    timestamp = int(m.group(1))
                    subs_data[timestamp] = int(m.group(2))
            else:
                print(f'unknown line: {line}')
    return throughput_data, subs_data

if len(sys.argv) == 1:
    print('lang?')
    exit()

lang = sys.argv[1]

cpudata, memdata, clidata, actdata = parse_data(lang)

beginning = min(clidata.keys())
ending = max(clidata.keys())

cpudata = {t-beginning: v for t, v in cpudata.items() if beginning <= t <= ending}
memdata = {t-beginning: v for t, v in memdata.items() if beginning <= t <= ending}
clidata = {t-beginning: v for t, v in clidata.items() if beginning <= t <= ending}
actdata = {t-beginning: v for t, v in actdata.items() if beginning <= t <= ending}

import matplotlib.pyplot as plt

x = range(min(clidata.keys()), max(clidata.keys()))

cpuusery   = [cpudata[t]['user'] for t in x]
cpusystemy = [cpudata[t]['system'] for t in x]
memy       = [memdata[t]['uss'] if t in memdata else 0 for t in x]

# maxy = max(*cpuusery, *cpusystemy)
# plt.plot(x, cpuusery, cpusystemy)

maxy = max(*memy)
plt.plot(x, memy)

for k, v in actdata.items():
    plt.axvline(x=k, color='red', ls=':', lw=1)
    plt.text(s=str(v), x=k, y=maxy * 12/13)

plt.show()