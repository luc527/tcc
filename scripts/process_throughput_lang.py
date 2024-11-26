import sys, os
from common import parse_cpu_data, parse_mem_data, parse_throughput_data, prepare_throughput_data, plot_cpu_usage, plot_mem_usage, plot_ticks
import pandas as pd
import matplotlib.pyplot as plt

try:
    lang = sys.argv[1]
    date = sys.argv[2]
    which = sys.argv[3]
except IndexError:
    print('usage: <lang> <date> <which graph>\n')
    exit()

if not os.path.exists('./data'):
    print('run this scipt at the root of the repository\n')
    exit()

test = 'throughput'
cli_ext = 'txt'

cpu_path = f'data/{lang}_cpu_{test}_{date}.csv'
mem_path = f'data/{lang}_mem_{test}_{date}.csv'
cli_path = f'data/{lang}_cli_{test}_{date}.{cli_ext}'

paths         = [cpu_path, mem_path, cli_path]
missing_paths = [path for path in paths if not os.path.exists(path)]
if missing_paths:
    for path in missing_paths:
        print(f'{path} not present')
    exit()

tru_data, act_data = parse_throughput_data(cli_path)
cpu_data = parse_cpu_data(cpu_path)
mem_data = parse_mem_data(mem_path)

x, tru_df, act_data, cpu_df, mem_df = prepare_throughput_data(tru_data, act_data, cpu_data, mem_data)

fig, ax = plt.subplots()

# TODO: which = delay, throughput, ...

ax.set_xlabel('Segundos após início do teste')
ax.grid(visible=True, axis='y')

if which == 'cpu':
    ax.set_title(f'Uso de CPU ({lang.capitalize()}, {test})')
    plot_cpu_usage(ax, x, cpu_df)
    plot_ticks(ax, act_data, tru_df.timestamp.max())
    ax.legend(['Usuário', 'Sistema', 'Inscrições por conexão'])
elif which == 'mem':
    ax.set_title(f'Uso de memória ({lang.capitalize()}, {test})')
    plot_mem_usage(ax, x, mem_df)
    plot_ticks(ax, act_data, tru_df.timestamp.max())
    ax.legend(['Memória', 'Inscrições por conexão'])
elif which == 'tru':
    x = x[10:]

    ax.set_title(f'Throughput por segundo (média, {lang.capitalize()})')

    # TODO: mean, min, p25, p50, p75, max
    # although maybe not per second (grouping by timestamp)
    # but more like each 5 seconds??
    tru_mean = tru_df.groupby('timestamp').median()
    tru_mean_psec = tru_mean.throughput_psec
    tru_mean_psec = tru_mean_psec.reindex(x, method='nearest')
    tru_mean_y = [tru_mean_psec[t] for t in x]
    ax.set_label('Throughput (msg / segundo / publicante)')
    ax.plot(x, tru_mean_y, color='tab:olive', linewidth=1)
    ax.set_ylim(bottom=0)
    ax.legend([''])
else:
    print(f'invalid which: {which}')
    exit()

fig.tight_layout()

plt.show()
# plt.savefig(f'graphs/{lang}_{test}_{which}.png', dpi=172)
# plt.close()
