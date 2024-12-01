import sys
import pandas as pd
from common import lines, lines_csv, parse_cpu_line, parse_mem_line
from collections import defaultdict
import matplotlib.pyplot as plt

lang    = None
compare = False
table   = False
date    = None
graph   = None
save    = False

yfactor = 10
ylim = None
percentile = None
cpu_which = None

def usage():
    print('wrong usage')
    exit()

try:
    for arg in sys.argv[1:]:
        [k, *v] = arg.split("=")
        match k:
            case "lang":
                lang = v[0]
            case "comparison":
                compare = True
            case "table":
                table = True
            case "date":
                date = v[0]
            case "graph":
                graph = v[0]
            case "show":
                save = False
            case "save":
                save = True
            case "yfactor":
                yfactor = int(v[0])
            case "ylim":
                ylim = int(v[0])
            case "p":
                percentile = int(v[0])
            case "cpu":
                cpu_which = v[0]
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

def latency_statistics(path):
    stats = defaultdict(list)
    label = None
    for line in lines(path):
        if line[0] == '#':
            label = line[1:]
        else:
            cols = line.split(',')
            stats[label].append({
                'timestamp': int(cols[0]),
                'latency': float(cols[1]) / 1000 / 1000
            })
    return stats

def get_lang_data(lang):
    cpu_path = f"data/latency_{lang}_cpu_{date}.csv"
    mem_path = f"data/latency_{lang}_mem_{date}.csv"
    lat_path = f"data/latency_{lang}_statistics_{date}.txt"
    itr_path = f"data/latency_{lang}_iters_{date}.csv"

    itr_data = []
    for line in lines(itr_path):
        cols = line.split(',')
        itr_data.append({
            'timestamp': int(cols[0]),
            'subscribers': int(cols[1]),
            'new_connections': int(cols[2]),
        })
    itr_df = pd.DataFrame(itr_data)

    cpu_data = [parse_cpu_line(l) for l in lines_csv(cpu_path)]
    mem_data = [parse_mem_line(l) for l in lines_csv(mem_path)]

    cpu_df = pd.DataFrame(cpu_data)

    mem_df = pd.DataFrame(mem_data)
    mem_df.uss /= (1024 * 1024)  # to mb

    stats = latency_statistics(lat_path)

    stats_dfs = {}
    for label, data in stats.items():
        df = pd.DataFrame(data)
        stats_dfs[label] = df

    return cpu_df, mem_df, stats_dfs, itr_df

fig, ax = plt.subplots()
ax.set_xlabel('Segundos após o início do teste')
ax.grid(visible='True', axis='y')

if table:

    # avg etc etc *per iteration*
    pass

elif compare:
    colors = {
        'go':      'tab:blue',
        'elixir':  'tab:purple',
        'node':    'tab:green',
    }
    langs = list(colors.keys())

    lang_dfs = {}
    for lang in langs:
        cpu_df, mem_df, stats_dfs, itr_df = get_lang_data(lang)
        timestamp_offset = min(
            itr_df.timestamp.min(),
            min(df.timestamp.min() for df in stats_dfs.values())
        )
        cpu_df.timestamp -= timestamp_offset
        mem_df.timestamp -= timestamp_offset
        itr_df.timestamp -= timestamp_offset
        
        for key, df in stats_dfs.items():
            df.timestamp -= timestamp_offset
            stats_dfs[key] = df

        lang_dfs[lang] = (
            {'cpu': cpu_df, 'mem': mem_df, 'itr': itr_df},
            stats_dfs
        )

    x = range(0, max(df.timestamp.max() for _, dfs in lang_dfs.values() for df in dfs.values()))

    for lang, (dfs0, dfs1) in lang_dfs.items():
        for key, df in dfs0.items():
            if key == 'itr':
                continue
            df = df.set_index('timestamp')
            df = df.reindex(x)
            dfs0[key] = df
        for key, df in dfs1.items():
            df = df.set_index('timestamp')
            df = df.reindex(x)
            dfs1[key] = df
        lang_dfs[lang] = (dfs0, dfs1)

    legend = []

    if graph == "cpu":
        ax.set_title('Comparação do uso de CPU (latência, média móvel de 5)')
        ax.set_ylabel('CPU (%)')
        for lang, (dfs, _) in lang_dfs.items():
            df = dfs['cpu']
            df = df.rolling(5).mean()
            Lang = lang.capitalize()
            if cpu_which == 'user':
                ax.plot(x, df.user, linewidth=1, linestyle='--', color=colors[lang])
                legend.append(f'{Lang}: usuário')
            else:
                ax.plot(x, df.system, linewidth=1, linestyle=':', color=colors[lang])
                legend.append(f'{Lang}: sistema')

    elif graph == "mem":
        ax.set_title('Comparação do uso de memória (latência)')
        ax.set_ylabel('Memória (mb)')
        for lang, (dfs, _) in lang_dfs.items():
            df = dfs['mem']
            ax.plot(x, df.uss, linewidth=1, linestyle='-', color=colors[lang])
            legend.append(f'{lang.capitalize()}')

    elif graph == "lat_mean":

        ax.set_title('Comparação da média de latência (média movel de 10)')
        ax.set_ylabel('Latência (segundos)')
        for lang, (_, dfs) in lang_dfs.items():
            df = dfs['mean']
            df = df.rolling(10).mean()
            ax.plot(x, df, linewidth=1, linestyle='-', color=colors[lang])
            legend.append(f'{lang.capitalize()}')

    elif graph == "lat_p":
        if not percentile:
            print(f'informe o percentil (argumento p=)')
            exit()
        description = 'a mediana' if percentile == 50 else f'o P{percentile}'
        key = 'median' if percentile == 50 else f'p{percentile}'

        ax.set_title(f'Comparação d{description} de latência (média móvel de 10)')
        ax.set_ylabel('Latência (segundos)')
        for lang, (_, dfs) in lang_dfs.items():
            df = dfs[key]
            df = df.rolling(10).mean()
            ax.plot(x, df, linewidth=1, linestyle='-', color=colors[lang])
            legend.append(f'{lang.capitalize()}')

    else:
        print(f'unknown or unimplemented graph "{graph}"')
        exit()

    ax.legend(legend)

else:
    cpu_df, mem_df, stats_dfs, itr_df = get_lang_data(lang)

    timestamp_offset = min(
        itr_df.timestamp.min(),
        min(df.timestamp.min() for df in stats_dfs.values()),
    )

    for label, df in stats_dfs.items():
        df.timestamp -= timestamp_offset
        stats_dfs[label] = df

    x = range(
        0,
        max(df.timestamp.max() for df in stats_dfs.values()) + 1
    )

    cpu_df.timestamp -= timestamp_offset
    cpu_df.set_index('timestamp', inplace=True)
    rcpu_df = cpu_df.rolling(5).mean()

    mem_offset = timestamp_offset
    mem_df.timestamp -= mem_offset
    mem_df.set_index('timestamp', inplace=True)

    itr_df.timestamp -= timestamp_offset
    itr_df.set_index('timestamp', inplace=True)

    y_max = None
    legend = []

    if graph == "cpu":
        y_max = max(rcpu_df.user.max(), rcpu_df.system.max())

        user_y = [rcpu_df.user[t]   for t in x]
        syst_y = [rcpu_df.system[t] for t in x]

        ax.set_title(f'Uso de cpu (latência, {lang.capitalize()}, média móvel de 5)')
        ax.set_ylabel('CPU (%)')
        ax.plot(x, user_y, linewidth=1, linestyle='--', color='black')
        ax.plot(x, syst_y, linewidth=1, linestyle=':', color='black')
        legend.append('Usuário')
        legend.append('Sistema')
    elif graph == "mem":
        y_max = mem_df.uss.max()

        mem_y = [mem_df.uss[t] if t in mem_df.uss else None for t in x]

        ax.set_title(f'Uso de memória (latência, {lang.capitalize()})')
        ax.plot(x, mem_y, linewidth=1, linestyle='-', color='black')
        ax.set_ylabel('Memória (mb)')
        legend.append('Memória')
    elif graph == "latency_mean":
        mean_df   = stats_dfs['mean']
        median_df = stats_dfs['median']

        y_max = max(mean_df.latency.max(), median_df.latency.max())

        mean_df   =   mean_df.set_index('timestamp').reindex(x, fill_value=0)
        median_df = median_df.set_index('timestamp').reindex(x, fill_value=0)

        mean_df   =   mean_df.rolling(5).mean()
        median_df = median_df.rolling(5).mean()

        mean_y   = [  mean_df.latency[t] if t in x else None for t in x]
        median_y = [median_df.latency[t] if t in x else None for t in x]

        ax.set_title(f'Latência (média móvel de 5, {lang.capitalize()})')
        ax.plot(x, mean_y, linewidth=1, linestyle='--', color='black')
        ax.plot(x, median_y, linewidth=1, linestyle=':', color='black')
        ax.set_ylabel('Latência (segundos)')
        legend.append('Média')
        legend.append('Mediana')

    elif graph == "latency_percentiles":

        median_df = stats_dfs['median']
        p90_df    = stats_dfs['p90']
        p95_df    = stats_dfs['p95']
        p99_df    = stats_dfs['p99']

        median_df = median_df.set_index('timestamp').reindex(x, fill_value=0)
        p90_df    =    p90_df.set_index('timestamp').reindex(x, fill_value=0)
        p95_df    =    p95_df.set_index('timestamp').reindex(x, fill_value=0)
        p99_df    =    p99_df.set_index('timestamp').reindex(x, fill_value=0)

        median_df = median_df.rolling(10).mean()
        p90_df    =    p90_df.rolling(10).mean()
        p95_df    =    p95_df.rolling(10).mean()
        p99_df    =    p99_df.rolling(10).mean()

        y_max = max(df.latency.max() for df in [median_df, p90_df, p95_df, p99_df])

        ax.set_title(f'Latência (percentis, média móvel de 10, {lang.capitalize()})')
        ax.set_ylabel('Latência (segundos)')
        ax.plot(x, median_df.latency, linewidth=1, linestyle='-', color='black')
        ax.plot(x, p90_df.latency, linewidth=1, linestyle='-.', color='black')
        ax.plot(x, p95_df.latency, linewidth=1, linestyle='--', color='black')
        ax.plot(x, p99_df.latency, linewidth=1, linestyle=':', color='black')
        legend.append('Mediana')
        legend.append('P90')
        legend.append('P95')
        legend.append('P99')

    else:
        print(f"unimplemented or unknown graph '{graph}'")
        exit()

    for t in itr_df.index:
        subs_per_topic = itr_df.subscribers[t]
        new_conns      = itr_df.new_connections[t]
        ax.axvline(t, color='tab:red', linewidth=1, linestyle=':')
        s = f'{subs_per_topic}'
        if new_conns:
            s = f'+{new_conns}c,\n{s}'
        y = y_max * (yfactor / (yfactor+1))
        ax.text(t, y, s, color='red', alpha=0.5)

    xticks = [0, *itr_df.index, x[-1]]
    nticks = len(xticks)
    if (xticks[nticks-1] - xticks[nticks-2]) < 10:
        xticks = xticks[:nticks-1]
    ax.set_xticks(xticks)

    legend.append('Inscrições por tópico')
    # mas quantas conexões por tópico? não é tão simples de calcular devido ao jeito como
    # o teste foi desenvolvido
    # TODO: talvez usar 'inscrições por tópico' também no throughput, p/ padronizar?
    ax.legend(legend)

ax.set_ylim(bottom=0)
if ylim:
    ax.set_ylim(top=ylim)

if save:
    path = f'graphs/latency_{'comparison' if compare else lang}_{graph}.png'
    if ylim:
        path = path.replace('.', f'_y{ylim}.')
    if percentile:
        path = path.replace('.', f'_p{percentile}.')
    if cpu_which:
        path = path.replace('.', f'_cpu-{cpu_which}.')
    plt.savefig(path, dpi=172)
    plt.close()
else:
    plt.show()