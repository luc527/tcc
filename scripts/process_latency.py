import sys
import pandas as pd
from common import lines, lines_csv, parse_cpu_line, parse_mem_line
import matplotlib.pyplot as plt

lang    = None
compare = False
date    = None
graph   = None
save    = False

def usage():
    print("usage: (lang=?|comparison) date=? graph=? (show|save)")
    exit()

try:
    for arg in sys.argv[1:]:
        [k, *v] = arg.split("=")
        match k:
            case "lang":
                lang = v[0]
            case "comparison":
                compare = True
            case "date":
                date = v[0]
            case "graph":
                graph = v[0]
            case "show":
                save = False
            case "save":
                save = True
except IndexError:
    usage()

if date is None:
    print("missing date")
    usage()

if graph is None:
    print("missing graph")
    usage()

if (lang is None) and (not compare):
    print("missing lang or 'comparison'")
    usage()

if lang is not None and lang not in "go node elixir".split(" "):
    print(f"unknown language '{lang}'")
    usage()

if compare:
    pass
else:
    cpu_path = f"data/latency_{lang}_cpu_{date}.csv"
    mem_path = f"data/latency_{lang}_mem_{date}.csv"
    lat_path = f"data/latency_{lang}_latencies_{date}.csv"
    itr_path = f"data/latency_{lang}_iters_{date}.csv"

    cpu_data = [parse_cpu_line(l) for l in lines_csv(cpu_path)]
    mem_data = [parse_mem_line(l) for l in lines_csv(mem_path)]

    cpu_df = pd.DataFrame(cpu_data)
    cpu_offset = cpu_df.timestamp.min()
    cpu_df.timestamp -= cpu_offset
    cpu_df.set_index('timestamp', inplace=True)
    rcpu_df = cpu_df.rolling(5).mean()

    mem_df = pd.DataFrame(mem_data)
    mem_offset =  mem_df.timestamp.min()
    mem_df.timestamp -= mem_offset
    mem_df.set_index('timestamp', inplace=True)
    mem_df.uss /= (1024 * 1024)  # to mb

    itr_data = []
    for line in lines(itr_path):
        cols = line.split(',')
        itr_data.append({
            'timestamp': int(cols[0]),
            'subscribers': int(cols[1]),
            'new_connections': int(cols[2]),
        })
    itr_df = pd.DataFrame(itr_data)

    fig, ax = plt.subplots()
    ax.set_xlabel('Segundos após o início do teste')
    ax.grid(visible='True', axis='y')

    timestamp_offset = None
    last_xtick = None
    y_max = None
    legend = []

    if graph == "cpu":
        timestamp_offset = cpu_offset
        last_xtick = cpu_df.index.max()
        y_max = max(rcpu_df.user.max(), rcpu_df.system.max())

        ax.set_title(f'Uso de cpu (latência, {lang.capitalize()}, média móvel de 5)')
        x = rcpu_df.index
        ax.plot(x, rcpu_df.user, linewidth=1, linestyle='--', color='black')
        legend.append('Usuário')
        ax.plot(x, rcpu_df.system, linewidth=1, linestyle=':', color='black')
        legend.append('Sistema')
        ax.set_ylabel('CPU (%)')
    elif graph == "mem":
        timestamp_offset = mem_offset
        last_xtick = mem_df.index.max()
        y_max = mem_df.uss.max()

        ax.set_title(f'Uso de memória (latência, {lang.capitalize()})')
        x = mem_df.index
        ax.plot(x, mem_df.uss, linewidth=1, linestyle='-', color='black')
        legend.append('Memória')
        ax.set_ylabel('Memória (mb)')
    else:
        print(f"unimplemented or unknown graph '{graph}'")
        exit()

    itr_df.timestamp -= timestamp_offset
    itr_df.set_index('timestamp', inplace=True)
    for t in itr_df.index:
        subs_per_topic = itr_df.subscribers[t]
        new_conns      = itr_df.new_connections[t]
        ax.axvline(t, color='tab:red', linewidth=1, linestyle=':')
        s = f'{subs_per_topic}'
        if new_conns:
            s = f'+{new_conns}c,\n{s}'
        ax.text(t, y_max * 30/31, s, color='red', alpha=0.5)
    ax.set_xticks([0, *itr_df.index, last_xtick])

    legend.append('Inscrições por tópico')
    # mas quantas conexões por tópico? não é tão simples de calcular devido ao jeito como
    # o teste foi desenvolvido
    # TODO: talvez usar 'inscrições por tópico' também no throughput, p/ padronizar?
    ax.legend(legend)

fig.tight_layout()

if save:
    plt.savefig(f'graphs/latency_{lang if lang else 'comparison'}_{graph}.png', dpi=172)
    plt.close()
else:
    plt.show()