import sys
import matplotlib.pyplot as plt
import numpy as np
from common import parse_throughput_data, parse_cpu_data, parse_mem_data, prepare_throughput_data, plot_ticks, plot_mem_usage

# TODO: force y axis starting at 0

try:
    date = sys.argv[1]
    which = sys.argv[2]
except IndexError:
    print(f'usage: <date> <which graph>\n')

colors = {
    'go': 'tab:blue',
    'elixir': 'tab:purple',
    'node': 'tab:green',
}

langs = list(colors.keys())
langscap = [lang.capitalize() for lang in langs]

dfs = {}

xs = []
iter_datas = []

for lang in langs:
    mps_data, delay_data, iter_data = parse_throughput_data(f'data/throughput_{lang}_cli_{date}.txt')
    cpu_data = parse_cpu_data(f'data/throughput_{lang}_cpu_{date}.csv')
    mem_data = parse_mem_data(f'data/throughput_{lang}_mem_{date}.csv')
    x, mps_df, delay_df, iter_data, cpu_df, mem_df = prepare_throughput_data(mps_data, iter_data, delay_data, cpu_data, mem_data)
    dfs[lang] = {
        'mps': mps_df,
        'cpu': cpu_df,
        'mem': mem_df,
    }
    xs.append(x)
    iter_datas.append(iter_data)

xmin = min(*(min(x) for x in xs))
xmax = max(*(max(x) for x in xs))
x = range(xmin, xmax+1)

for lang, dic in dfs.items():
    dfs[lang]['cpu'] = dfs[lang]['cpu'].reindex(x, method='nearest')
    dfs[lang]['mem'] = dfs[lang]['mem'].reindex(x, method='nearest')

iter_data = iter_datas[0]

fig, ax = plt.subplots()

ax.grid(visible=True, axis='y')
ax.set_xlabel('Segundos após início do teste')

if which == 'mem':
    ax.set_title(f'Uso de memória no teste de throughput')

    for lang in langs:
        color = colors[lang]
        mem_y = [dfs[lang]['mem']['uss'][t] / 1024 / 1024 for t in x]
        ax.plot(x, mem_y, color=color, linewidth=1)

    ax.set_ylabel('Memória (mb)')
    ax.set_ylim(bottom=0)
    ax.tick_params(axis='y')
    ax.set_yticks(np.arange(0, 425, 25))
    plot_ticks(ax, iter_data, xmax)
    ax.legend([*langscap, 'Inscritos por tópico'])

elif which == 'cpu':
    ax.set_title(f'Uso de CPU no teste de throughput (média móvel de 5)')

    legend = []
    for lang in langs:
        color = colors[lang]
        cpu_usr_y = [dfs[lang]['cpu']['user'][t] for t in x]
        cpu_sys_y = [dfs[lang]['cpu']['system'][t] for t in x]
        ax.plot(x, cpu_usr_y, color=color, linewidth=1, linestyle='--')
        ax.plot(x, cpu_sys_y, color=color, linewidth=1, linestyle=':')
        legend.append(f'{lang.capitalize()}, usuário')
        legend.append(f'{lang.capitalize()}, sistema')

    ax.set_ylabel('CPU (%)')
    ax.set_ylim(bottom=0)
    ax.tick_params(axis='y')
    plot_ticks(ax, iter_data, xmax)
    legend.append('Inscritos por tópico')
    ax.legend(legend)

elif which == 'tru':

    ax.set_title(f'Mensagens/segundo (média móvel de 10)')
    legend = []
    for lang in langs:
        color = colors[lang]
        mps = dfs[lang]['mps']
        cps = mps.groupby('timestamp')['count'].sum()
        x = x[8:]
        cps = cps.reindex(x, fill_value=0)

        # rcps = cps
        # rcps = cps.rolling(10).mean()
        rcps = cps.rolling(10).mean()
        # rcps = cps.rolling(30).mean()

        y = [rcps[t] for t in x]
        ax.plot(x, y, color=color, linewidth=1)
        legend.append(lang.capitalize())
    ax.set_ylabel('Mensagens enviadas')
    plot_ticks(ax, iter_data, xmax)
    legend.append('Inscritos por tópico')
    ax.legend(legend)

elif which == 'trucum':

    ax.set_title('Mensagens/segundo (cumulativo)')
    legend = []
    for lang in langs:
        color = colors[lang]
        mps = dfs[lang]['mps']
        cps = mps.groupby('timestamp')['count'].sum()
        cps = cps.reindex(x, fill_value=0)
        sps = cps.cumsum()
        y = [sps[t] for t in x]
        ax.plot(x, y, color=color, linewidth=1)
        legend.append(lang.capitalize())
    ax.set_ylabel('Número de mensagens enviadas')
    ax.ticklabel_format(style='plain')

    legend.append('Inscritos por tópico')
    ax.legend(legend)

    plot_ticks(ax, iter_data, xmax)

ax.set_ylim(bottom=0)
plt.savefig(f'graphs/throughput_comparison_{which}.png', dpi=172)
plt.close()
# plt.show()