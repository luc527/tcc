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

cpu_path = f'data/throughput_{lang}_cpu_{date}.csv'
mem_path = f'data/throughput_{lang}_mem_{date}.csv'
cli_path = f'data/throughput_{lang}_cli_{date}.txt'

# TODO: fazer tb tabelas, média de cpu e memória e throughput dentro ao longo de cada etapa

paths         = [cpu_path, mem_path, cli_path]
missing_paths = [path for path in paths if not os.path.exists(path)]
if missing_paths:
    for path in missing_paths:
        print(f'{path} not present')
    exit()

cpu_data = parse_cpu_data(cpu_path)
mem_data = parse_mem_data(mem_path)
mps_data, delay_data, iter_data = parse_throughput_data(cli_path)

x, mps_df, delay_df, iter_data, cpu_df, mem_df = prepare_throughput_data(mps_data, iter_data, delay_data, cpu_data, mem_data)

fig, ax = plt.subplots()

ax.set_xlabel('Segundos após início do teste')
ax.grid(visible=True, axis='y')

if which == 'cpu':
    ax.set_title(f'Uso de CPU (throughput, {lang.capitalize()}, média móvel de 5)')
    plot_cpu_usage(ax, x, cpu_df)
    y_max = max(cpu_df.user.max(), cpu_df.system.max())
    plot_ticks(ax, iter_data, mps_df.timestamp.max(), y_max)
    ax.legend(['Usuário', 'Sistema', 'Inscrições por tópico'])
elif which == 'mem':
    ax.set_title(f'Uso de memória ({lang.capitalize()}, throughput)')
    plot_mem_usage(ax, x, mem_df)
    y_max = mem_df.uss.max()
    plot_ticks(ax, iter_data, mps_df.timestamp.max(), y_max)
    ax.legend(['Memória', 'Inscrições por tópico'])
elif which == 'tru':
    ax.set_title(f'Throughput ({lang.capitalize()}, média móvel de 10)')

    mps_df = mps_df[mps_df['timestamp'] > 8]
    x = x[8:]

    cps = mps_df.groupby('timestamp')['count'].sum()
    cps = cps.reindex(x, fill_value=0)
    sps = cps.cumsum()
    rcps = cps.rolling(10).mean()

    color = 'black'
    ax.set_ylabel('Mensagens por segundo')
    # ax.tick_params('y', labelcolor=color)

    y = [rcps[t] for t in x]
    ax.plot(x, y, color=color, linewidth=1)

    # y = [sps[t] for t in x]
    # color = 'black'
    # ax2 = ax.twinx()
    # ax2.set_ylabel('Mensagens enviadas (cumulativo)', color=color)
    # ax2.tick_params('y', labelcolor=color)
    # ax2.plot(x, y, linewidth=1, color=color)

    plot_ticks(ax, iter_data, mps_df.timestamp.max(), max(y))
else:
    print(f'invalid which: {which}')
    exit()

ax.set_ylim(bottom=0)
plt.savefig(f'graphs/throughput_{lang}_{which}.png', dpi=172)
plt.close()

# plt.show()
