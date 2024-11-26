import re
import pandas as pd
from collections import defaultdict

def parse_cpu_data(path):
    data = {}
    with open(path, 'r') as f:
        f.readline() # ignore fst line (header)
        for line in f:
            line = line.rstrip()
            cols = line.split(',')
            timestamp = int(cols[0])
            data[timestamp] = {
                'user': float(cols[1]),
                'system': float(cols[2]),
            }
    return data

def parse_mem_data(path):
    data = {}
    with open(path, 'r') as f:
        f.readline() # ignore fst line (header)
        for line in f:
            line = line.rstrip()
            cols = line.split(',')
            timestamp = int(cols[0])
            data[timestamp] = {
                'uss': int(cols[1]),
            }
    return data

def parse_throughput_pub_line(line):
    pat = r'pub: (\d+) topic=(\d+) pub=(\d+) msg=\d+ delayMs=(\d+) frame=(\d+) throughputPsec=([\d.]+)'
    m = re.match(pat, line)
    return {
        'timestamp': int(m.group(1)),
        'topic': int(m.group(2)),
        'publisher': int(m.group(3)),
        'delay_ms': int(m.group(4)),
        'frame': int(m.group(5)),
        'throughput_psec': float(m.group(6)),
    } if m else None

def parse_throughput_data(path):
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
                pat = r'dbg: (\d+) iteration (\d+), topics per conn (\d+)'
                m = re.match(pat, line)
                if m:
                    timestamp = int(m.group(1))
                    subs_data[timestamp] = {
                        'iteration': int(m.group(2)),
                        'topics_per_conn': int(m.group(3)),
                    }
            else:
                print(f'unknown line: {line}')
    return throughput_data, subs_data

def parse_latency_line(line):
    [timestamp_sec, topic, latency_usec] = line.split(',')
    return int(timestamp_sec), int(latency_usec)

def parse_latency_data(path):
    latency_data = defaultdict(list)
    with open(path, 'r') as f:
        for line in f:
            line = line.rstrip()
            timestamp_sec, latency_usec = parse_latency_line(line)
            latency_data[timestamp_sec].append(latency_usec / 1000 / 1000) # now in seconds
    return latency_data

def reindex(dic, beginning, ending):
    return {t-beginning: v 
            for t, v in dic.items()
            if beginning <= t <= ending}

def to_y(dic, ran, f=None):
    lis = []
    prev = None
    # forward fill
    for i in ran:
        if i in dic:
            val = f(dic[i]) if f else dic[i]
            lis.append(val)
            prev = val
        else:
            lis.append(prev)
    return lis

def to_df(dic):
    lis = [{'timestamp': t, **v} for t, v in dic.items()]
    return pd.DataFrame(lis)

def prepare_throughput_data(tru_data, act_data, cpu_data, mem_data):
    beginning = min(tru_data.keys())
    ending    = max(tru_data.keys())
    tru_data  = reindex(tru_data, beginning, ending)
    act_data  = reindex(act_data, beginning, ending)
    cpu_df = to_df(reindex(cpu_data, beginning, ending))
    mem_df = to_df(reindex(mem_data, beginning, ending))

    tru_data_degrouped = [{'timestamp': t, **v}
                        for t, vs in tru_data.items()
                        for v in vs]

    tru_df = pd.DataFrame(tru_data_degrouped)
    tru_df.loc[tru_df['throughput_psec'] >= 1000, 'throughput_psec'] = float('nan')
    tru_df.bfill(inplace=True)

    cpu_df = cpu_df.rolling(5).mean().bfill()

    x = range(tru_df.timestamp.min(), tru_df.timestamp.max()+1)

    cpu_df = cpu_df.reindex(x, method='nearest')
    mem_df = mem_df.reindex(x, method='nearest')

    return x, tru_df, act_data, cpu_df, mem_df

def plot_cpu_usage(ax, x, cpu_df):
    color = 'tab:blue'
    ax.set_ylabel('CPU (%)')
    cpu_usr_y = [cpu_df['user'][t] for t in x]
    cpu_sys_y = [cpu_df['system'][t] for t in x]
    ax.plot(x, cpu_usr_y, linewidth=1, linestyle='--', color=color)
    ax.plot(x, cpu_sys_y, linewidth=1, linestyle=':', color=color)
    ax.set_ylim(bottom=0)
    ax.tick_params(axis='y')

def plot_mem_usage(ax, x, mem_df, color='tab:green'):
    mem_y = [mem_df['uss'][t] / 1024 / 1024 for t in x]
    ax.set_ylabel('Memória (mb)')
    ax.plot(x, mem_y, color=color, linewidth=1)
    ax.set_ylim(bottom=0)
    ax.tick_params(axis='y')

def plot_ticks(ax, act_data, max):
    color = 'tab:red'
    ticks = [0, *act_data.keys(), max]
    ax.set_xticks(ticks)
    for line_x, o in act_data.items():
        ax.axvline(line_x, color=color, linewidth=1, linestyle=':')
        ax.text(line_x+1, 0.2, f'{o['topics_per_conn']}', color=color)