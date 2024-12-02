import sys
import pandas as pd
import numpy as np
import matplotlib.pyplot as plt
from common import darken, lines_csv, parse_cpu_line, parse_mem_line, parse_throughput_data, degroup_throughput_data, subscribers_at
from collections import defaultdict


try:
    date = sys.argv[1]
    which = len(sys.argv) > 2 and sys.argv[2]
    save = len(sys.argv) > 3 and sys.argv[3] == 'save'
except IndexError:
    print(f'wrong usage')
    exit()

langs = ['go', 'elixir', 'node']

table_df = pd.DataFrame()

for lang in langs:
    cpu_path = f'data/throughput_{lang}_cpu_{date}.csv'
    mem_path = f'data/throughput_{lang}_mem_{date}.csv'
    mps_path = f'data/throughput_{lang}_cli_{date}.txt'

    cpu_data = list(map(parse_cpu_line, lines_csv(cpu_path)))
    mem_data = list(map(parse_mem_line, lines_csv(mem_path)))
    mps_data, _, iter_data = parse_throughput_data(mps_path)
    mps_data = degroup_throughput_data(mps_data)

    tru_data = defaultdict(list)
    for obj in mps_data:
        tru_data[obj['timestamp']].append(obj['count'])
    tru_data = [{'timestamp': k, 'count': sum(v)} for k, v in tru_data.items()]

    beg = min(obj['timestamp'] for obj in tru_data)
    end = max(obj['timestamp'] for obj in tru_data)

    cpu_data = [
        {
            'user': d['user'],
            'system': d['system'],
            'iter': subscribers_at(d, iter_data),
        }
        for d in cpu_data
        if beg <= d['timestamp'] <= end
    ]

    mem_data = [
        {
            'uss': d['uss'],
            'iter': subscribers_at(d, iter_data),
        }
        for d in mem_data
        if beg <= d['timestamp'] <= end 
    ]

    tru_data = [
        {
            'count': d['count'],
            'iter': subscribers_at(d, iter_data),
        }
        for d in tru_data
        if beg <= d['timestamp'] <= end
    ]

    cpu_df = pd.DataFrame(cpu_data)
    mem_df = pd.DataFrame(mem_data)
    tru_df = pd.DataFrame(tru_data)

    cpu_mean_df = cpu_df.groupby('iter').mean()
    cpu_std_df  = cpu_df.groupby('iter').std()

    mem_mean_df = mem_df.groupby('iter').mean()
    mem_std_df  = mem_df.groupby('iter').std()
    mem_last_df = mem_df.groupby('iter').agg(lambda a: a[a.index[-1]])

    tru_sum_df  = tru_df.groupby('iter').sum() # total messages sent per iteration
    tru_mean_df = tru_df.groupby('iter').mean()
    tru_std_df  = tru_df.groupby('iter').std()

    table_df[f'{lang}:cpu.user.mean']   = cpu_mean_df.user
    table_df[f'{lang}:cpu.user.std']    = cpu_std_df.user
    table_df[f'{lang}:cpu.system.mean'] = cpu_mean_df.system
    table_df[f'{lang}:cpu.system.std']  = cpu_std_df.system
    table_df[f'{lang}:mem.uss.mean']    = mem_mean_df.uss
    table_df[f'{lang}:mem.uss.std']     = mem_std_df.uss
    table_df[f'{lang}:mem.uss.last']    = mem_last_df.uss
    table_df[f'{lang}:tru.count.sum']   = tru_sum_df['count']
    table_df[f'{lang}:tru.count.mean']  = tru_mean_df['count']
    table_df[f'{lang}:tru.count.std']   = tru_std_df['count']

    if 'iter' not in table_df.columns:
        table_df['iter'] = [0, *iter_data.values()]

table_df.set_index('iter', inplace=True)
cols = set(col.split(':')[1] for col in table_df.columns)

cpu_cols = [col for col in cols if col.startswith('cpu')]
mem_cols = [col for col in cols if col.startswith('mem')]
tru_cols = [col for col in cols if col.startswith('tru')]

iters = table_df.index

if which == "table":
    print('\n\ncpu')
    for it in iters:
        print(it, end='')
        for lang in langs:
            for col in ['user', 'system']:
                mean = table_df[f'{lang}:cpu.{col}.mean'][it]
                std  = table_df[f'{lang}:cpu.{col}.std'][it]
                print(f' & ${mean:.2f} \\pm {std:.2f}$', end='')
        print(' \\\\', end='')
        # print('\\hline', end='')
        print()

    print('\n\nmem')
    for it in iters:
        print(it, end='')
        for lang in langs:
            last = table_df[f'{lang}:mem.uss.last'][it] / 1024 / 1024
            print(f' & {last:.1f}', end='')
        print(' \\\\', end='')
        print()

    print('\n\ntru')
    for it in iters:
        print(it, end='')
        for lang in langs:
            mean = table_df[f'{lang}:tru.count.mean'][it]
            std  = table_df[f'{lang}:tru.count.std'][it]
            print(f' & ${mean:.1f} \\pm {std:.1f}$', end='')
        for lang in langs:
            tot = table_df[f'{lang}:tru.count.sum'][it]
            print(f' & {tot}', end='')
        print(' \\\\', end='')
        print()

    go_tot = table_df['go:tru.count.sum'].sum()
    ex_tot = table_df['elixir:tru.count.sum'].sum()
    no_tot = table_df['node:tru.count.sum'].sum()
    print(f'Tot & -- & -- & -- & {go_tot} & {ex_tot} & {no_tot} \\\\')

    exit()


fig, ax = plt.subplots()

ax.grid(visible=True, axis='y', zorder=0)
ax.set_xticks(iters)
ax.set_xlabel('Número de inscritos por tópico')

lang_colors = {
    'go': 'tab:blue',
    'elixir': 'tab:purple',
    'node': 'tab:green',
}

if which == 'cpu':
    ax.set_yticks(np.arange(0, 55, 5))
    ax.set_title('Média do uso de CPU no teste de throughput')
    ax.set_ylabel('CPU (%)')
    for lang_offset, lang in zip((-1.1, 0, 1.1), langs):
        for kind_offset, kind in zip((-0.25, +0.25), ['system', 'user']):
            color = lang_colors[lang]
            if kind == 'user':
                color = darken(color)
                
            x = iters
            y = table_df[f'{lang}:cpu.{kind}.mean']
            ax.bar(
                x + lang_offset + kind_offset,
                y,
                color=color,
                label=f'{lang.capitalize()} ({'usuário' if kind == 'user' else 'sistema'})',
                zorder=3,
                width=0.5,
            )

elif which == 'mem':
    ax.set_title('Uso de memória no teste de throughput')
    ax.set_ylabel('Memória (mB)')
    ax.set_yticks(np.arange(0, 400+25, 25))
    for lang_offset, lang in zip((-1, 0, 1), langs):
        color = lang_colors[lang]
        x = iters
        y = table_df[f'{lang}:mem.uss.last'] / 1024 / 1024
        ax.bar(
            x + lang_offset,
            y,
            color=color,
            label=lang.capitalize(),
            zorder=3,
            width=0.9,
        )

elif which == 'tru':
    ax.set_title('Média de mensagens/segundo')
    ax.set_ylabel('Mensagens/segundo')
    ax.set_yticks(np.arange(0, 700+50, 50))
    for lang_offset, lang in zip((-1, 0, 1), langs):
        color = lang_colors[lang]
        x = iters[1:]
        y = table_df[f'{lang}:tru.count.mean']
        y = y.reindex(x)
        ax.bar(
            x + lang_offset,
            y,
            color=color,
            label=lang.capitalize(),
            zorder=3,
            width=0.9,
        )

elif which == 'trucum':
    ax.set_title('Total de mensagens enviadas no teste de throughput')
    ax.set_ylabel('Número de mensagens enviadas')
    for lang_offset, lang in zip((-1, 0, 1), langs):
        color = lang_colors[lang]
        x = iters
        y = table_df[f'{lang}:tru.count.sum']
        ax.bar(
            x + lang_offset,
            y,
            color=color,
            label=lang.capitalize(),
            zorder=3,
            width=0.9,
        )


ax.legend()

if save:
    plt.savefig(f'graphs/throughput_comparison_bar_{which}.png', dpi=172)
    plt.close()
else:
    plt.show()

