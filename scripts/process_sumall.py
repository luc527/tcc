import re
import sys
import numpy as np
import pandas as pd
import matplotlib.pyplot as plt
from common import lines, lines_csv, parse_cpu_line, parse_mem_line, darken
from pprint import pprint

# TODO: escrever no texto do tcc que são em segundos as tabelas de latência

def parse_sumall_file(path):
    text_latencies = []
    sum_latencies = []
    iterations = []

    for line in lines(path):
        if line.startswith('sub'):
            pat = r'sub: \d+ send=(\d+) latency=(\d+), kind=(\w+)'
            if (m := re.match(pat, line)):
                obj = {
                    'timestamp': int(m.group(1)) // 1000 // 1000,
                    'latency': float(m.group(2)),  # let's keep it in microseconds for now
                }
                kind = m.group(3)
                if kind == 'text':
                    text_latencies.append(obj)
                else:
                    sum_latencies.append(obj)

            else:
                # print(f'line "{line}" did not match')
                pass

        elif line.startswith('dbg'):
            pat = r'dbg: (\d+) sumlen=(\d+)'
            if (m := re.match(pat, line)):
                obj = {
                    'timestamp': int(m.group(1)) // 1000 // 1000,
                    'len': int(m.group(2)),
                }
                iterations.append(obj)

    return text_latencies, sum_latencies, iterations

def find_iter(o, iters):
    prev = 0
    for it in iters:
        if o['timestamp'] < it['timestamp']:
            break
        prev = it['len']
    return prev

def fix_timestamp(data, beg, end):
    return [
        {**o, 'timestamp': o['timestamp']-beg}
        for o in data
        if beg <= o['timestamp'] <= end
    ]

def prtex(a, line=False):
    print(' & '.join(map(str, a)), end='')
    print(' \\\\', end='')
    if line:
        print(' \\hline', end='')
    print()


graph = None
which_lat = None
which_metrics = None
save = False

try:
    for s in sys.argv[1:]:
        a = s.split('=')
        if len(a) == 2:
            [k, v] = a
        else:
            [k,] = a
        if k == 'graph':
            graph = v
        elif k == 'which':
            which_lat = v
        elif k == 'save':
            save = True
        elif k == 'metrics':
            which_metrics = v
        else:
            print(f'unknown option "{k}" with value {v}')
except:
    print('wrong usage')
    exit()

lang_colors = {
    'go': 'tab:blue',
    'elixir': 'tab:purple',
    'node': 'tab:green',
}
langs = list(lang_colors.keys())

D = {}

for lang in langs:
    date = '3dez3' if lang == 'elixir' else '3dez2'
    cpu_path = f'data/sumall_{lang}_cpu_{date}.csv'
    mem_path = f'data/sumall_{lang}_mem_{date}.csv'
    cli_path = f'data/sumall_{lang}_cli_{date}.txt'

    text_lats, sum_lats, iters = parse_sumall_file(cli_path)

    cpu_data = [parse_cpu_line(line) for line in lines_csv(cpu_path)]
    mem_data = [parse_mem_line(line) for line in lines_csv(mem_path)]

    cpu_data  = [{'iter': find_iter(o, iters), **o} for o in cpu_data]
    mem_data  = [{'iter': find_iter(o, iters), **o} for o in mem_data]
    text_lats = [{'iter': find_iter(o, iters), **o} for o in text_lats]
    sum_lats  = [{'iter': find_iter(o, iters), **o} for o in sum_lats]

    beg = min(obj['timestamp'] for objs in [text_lats,sum_lats,iters] for obj in objs)
    end = max(obj['timestamp'] for objs in [text_lats,sum_lats,iters] for obj in objs)

    cpu_data  = fix_timestamp(cpu_data, beg, end)
    mem_data  = fix_timestamp(mem_data, beg, end)
    text_lats = fix_timestamp(text_lats, beg, end)
    sum_lats  = fix_timestamp(sum_lats, beg, end)

    iters = [{
        'timestamp': obj['timestamp'] - beg,
        'len': obj['len']
    } for obj in iters]

    cpu_df = pd.DataFrame(cpu_data)
    mem_df = pd.DataFrame(mem_data)
    txlat_df = pd.DataFrame(text_lats)
    smlat_df = pd.DataFrame(sum_lats)

    D[lang] = {
        'cpu': cpu_df,
        'mem': mem_df,
        'txlat': txlat_df,
        'smlat': smlat_df,
        'iters': iters,
    }

iters = [100, 400, 700, 1000, 1300]

if graph == 'mem_table':
    lang_dfs = {
        lang: dfs['mem'].groupby('iter').agg(lambda a: a[a.index[-1]])
        for lang, dfs in D.items()
    }

    prtex(['Tam', *(lang.capitalize() + ' (mb)' for lang in lang_dfs.keys())], line=True)

    for iter in iters:
        vals = [
            iter,
            *(f'{df['uss'][iter]/1024/1024:.2f}' for df in lang_dfs.values())
        ]
        prtex(vals)

elif graph == 'cpu_table':

    cols = ['user', 'system']
    aggs = ['mean', 'std']

    lang_dfs = {
        lang: {
            f'{col}.{agg}': getattr(dfs['cpu'].groupby('iter'), agg)()[col]
            for col in cols
            for agg in aggs
        }
        for lang, dfs in D.items()
    }

    prtex([
        'Tam',
        *(lang.capitalize() + ('Us' if col == 'user' else 'Sis') + '\\%'
          for lang in lang_dfs.keys()
          for col in cols
        )
    ], line=True)

    for iter in iters:
        prtex([
            iter,
            *(f' ${dfs[f'{col}.mean'][iter]:.2f} \\pm {dfs[f'{col}.std'][iter]:.2f}$ '
                for dfs in lang_dfs.values()
                for col in cols
            )
        ])

elif graph == 'lat_table':

    if which_lat not in ('text', 'sum'):
        print(f'missing which_lat=')
        exit()

    idx = 'txlat' if which_lat == 'text' else 'smlat'

    lang_series = {
        lang: dfs[idx].groupby('iter')['latency']
        for lang, dfs in D.items()
    }

    aggs_groups = [
        {
            'mean': lambda s: s.mean().reindex(iters),
            'median': lambda s: s.median().reindex(iters),
        },
        {
            'p95': lambda s: s.quantile(0.95).reindex(iters),
            'p99': lambda s: s.quantile(0.99).reindex(iters),
        }
    ]

    labels = {
        'mean': 'Média',
        'median': 'Mediana',
        'p95': 'P95',
        'p99': 'P99',
    }

    divby = 1000 if which_lat == 'text' else 1_000_000
    desc = 'ms' if which_lat == 'text' else 's'

    for aggs in aggs_groups:
        print('\n\n')

        prtex([
            'Tam',
            *(f'{lang.capitalize()}, {labels[agg_name]} ({desc})'
                for agg_name in aggs.keys()
                for lang in lang_series.keys()
            )
        ])

        for iter in iters:
            prtex([
                iter,
                *(f'{agg_fn(series)[iter]/divby:.2f}'
                    for agg_fn in aggs.values()
                    for series in lang_series.values()
                ),
            ])

else:

    x = np.arange(1, len(iters)+1)

    fig, ax = plt.subplots()
    ax.set_xticks(x)
    ax.set_xticklabels(iters)
    ax.set_xlim(left=0.25, right=len(iters)+0.75)
    ax.set_xlabel('Tamanho da publicação (número de caracteres)')
    ax.grid(visible=True, axis='y')


    if graph == 'mem':
        ax.set_title('Uso de memória no teste de tarefas CPU-intensivas')
        ax.set_ylabel('Memória (mb)')

        for offset, lang in zip((-0.28, 0, +0.28), langs):
            color = lang_colors[lang]
            y = D[lang]['mem'].groupby('iter').agg(lambda a: a[a.index[-1]])['uss']
            y = y / 1024 / 1024
            ax.bar(x+offset, y, color=color, label=lang.capitalize(), width=0.25, zorder=3)

    elif graph == 'cpu':
        ax.set_title('Uso de CPU no teste de tarefas CPU-intensivas')
        ax.set_ylabel('CPU (%)')

        for lang_offset, lang in zip((-1, 0, +1), langs):
            # for kind_offset, kind in zip((-1, +1), ['system', 'user']):
            #     color = lang_colors[lang]
            #     if kind == 'user':
            #         color = darken(color)

            #     y = D[lang]['cpu'].groupby('iter').mean()[kind]
            #     ax.bar(
            #         x + 0.25*lang_offset + 0.05*kind_offset,
            #         y,
            #         width=0.1,
            #         color=color,
            #         zorder=3,
            #         label=f'{lang.capitalize()} ({'usuário' if kind == 'user' else 'sistema'})',
            #     )

            color = lang_colors[lang]
            y = D[lang]['cpu'].groupby('iter').mean()['user']
            ax.bar(
                x + 0.28*lang_offset,
                y,
                width=0.25,
                color=color,
                zorder=3,
                label=f'{lang.capitalize()} (usuário)',
            )

    elif graph == 'lat':
        if which_lat not in ('text', 'sum'):
            print(f'missing which=')
            exit()

        if which_metrics not in ('mean', 'perc'):
            print(f'missing metrics=')
            exit()

        idx = 'txlat' if which_lat == 'text' else 'smlat'
        dfs = {lang: dfs[idx] for lang, dfs in D.items()}

        metrics = ['mean', 'median'] if which_metrics == 'mean' else ['p95', 'p99']

        calc_metrics = {
            'mean': lambda df: df.groupby('iter').mean()['latency'],
            'median': lambda df: df.groupby('iter').median()['latency'],
            'p95': lambda df: df.groupby('iter').quantile(0.95)['latency'],
            'p99': lambda df: df.groupby('iter').quantile(0.99)['latency'],
        }

        label_metrics = {
            'mean': 'Média',
            'median': 'Mediana',
            'p95': 'P95',
            'p99': 'P99'
        }

        ax.set_title(f'Latência de mensagens {'de texto' if which_lat == 'text' else 'de computação'} no teste de tarefas CPU-intensivas')
        ax.set_ylabel(f'Latência ({'milissegundos' if which_lat == 'text' else 'segundos'})')

        for l_offset, lang in zip((-1, 0, +1), langs):
            for m_offset, metric in zip((-1, 1), metrics):
                y = calc_metrics[metric](dfs[lang]) / 1000

                if which_lat == 'sum':
                    y /= 1000

                y = y.reindex(iters, fill_value=0)

                color = lang_colors[lang]
                if m_offset == -1:
                    color = darken(color)

                ax.bar(
                    x + 0.25*l_offset + 0.05*m_offset,
                    y,
                    width=0.1,
                    color=color,
                    label=f'{lang.capitalize()} ({label_metrics[metric]})',
                    zorder=3,
                )

        if which_lat == 'sum':
            y_max = max(*(
                calc_metrics[metric](df).max() / 1000 / 1000
                for lang, df in dfs.items()
                for metric in metrics
                if lang != 'elixir'
            ))
            ax.set_ylim(top=y_max+1)

            elixirs = {
                metric: calc_metrics[metric](dfs['elixir']) / 1000 / 1000
                for metric in metrics
            }
            a = elixirs[metrics[0]][700]
            b = elixirs[metrics[1]][700]

            ax.text(s=f'{a:.2f}', x=3-0.7, y=y_max-1, color=darken('tab:purple'))
            ax.text(s=f'{b:.2f}', x=3+0.15, y=y_max-1, color='tab:purple')

    if graph=='lat' and which_lat=='text' and which_metrics=='mean':
        ax.legend(loc='lower left')
    elif graph=='lat' and which_lat=='text' and which_metrics=='perc':
        ax.legend(loc='lower right')
    else:
        ax.legend()

    if save:
        plt.savefig(f'graphs/sumall_{graph}{which_lat or ''}{which_metrics or ''}_{date}.png', dpi=172)
        plt.close()
    else:
        plt.show()