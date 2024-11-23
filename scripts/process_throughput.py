import sys
import glob

from common import parse_cpu_data, parse_mem_data, parse_throughput_data, reindex, to_y

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
    trudata, subdata = parse_throughput_data(clipath)
    cpudata = parse_cpu_data(cpupath)
    memdata = parse_mem_data(mempath)
    
    return cpudata, memdata, trudata, subdata


if len(sys.argv) == 1:
    print('lang?')
    exit()

lang = sys.argv[1]

cpudata, memdata, clidata, actdata = parse_data(lang)

beginning = min(clidata.keys())
ending = max(clidata.keys())

cpudata = reindex(cpudata, beginning, ending)
memdata = reindex(memdata, beginning, ending)
clidata = reindex(clidata, beginning, ending)
actdata = reindex(actdata, beginning, ending)

import matplotlib.pyplot as plt

x = range(min(clidata.keys()), max(clidata.keys()))

# cpuusery   = to_y(cpudata, x, lambda d: d['user'])
# cpusystemy = to_y(cpudata, x, lambda d: d['system'])
# memy       = to_y(memdata, x, lambda d: d['uss'])
# truy       = to_y(clidata, x, lambda d: d['throughput_psec'])

# # maxy = max(*cpuusery, *cpusystemy)
# # plt.plot(x, cpuusery, cpusystemy)

# maxy = max(*memy)
# plt.plot(x, memy)

# for k, v in actdata.items():
#     plt.axvline(x=k, color='red', ls=':', lw=1)
#     plt.text(s=str(v), x=k, y=maxy * 12/13)

# plt.show()

import pandas as pd