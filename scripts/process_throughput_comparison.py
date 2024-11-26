import sys
import matplotlib.pyplot as plt
from common import parse_throughput_data, parse_cpu_data, parse_mem_data, prepare_throughput_data, plot_ticks, plot_mem_usage

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

dfs = {}

xs = []
act_datas = []

for lang in langs:
    tru_data, act_data = parse_throughput_data(f'data/{lang}_cli_throughput_{date}.txt')
    cpu_data = parse_cpu_data(f'data/{lang}_cpu_throughput_{date}.csv')
    mem_data = parse_mem_data(f'data/{lang}_mem_throughput_{date}.csv')
    x, tru_df, act_data, cpu_df, mem_df = prepare_throughput_data(tru_data, act_data, cpu_data, mem_data)
    dfs[lang] = {
        'tru': tru_df,
        'cpu': cpu_df,
        'mem': mem_df,
    }
    xs.append(x)
    act_datas.append(act_data)


xmin = min(*(min(x) for x in xs))
xmax = max(*(max(x) for x in xs))
x = range(xmin, xmax+1)

for lang, dic in dfs.items():
    for k, df in dic.items():
        dfs[lang][k] = df.reindex(x, method='nearest')

act_data = act_datas[0]

fig, ax = plt.subplots()

ax.grid(visible=True, axis='y')

if which == 'mem':
    ax.set_title(f'Comparação do uso de memória (throughput)')

    for lang in langs:
        color = colors[lang]
        mem_y = [dfs[lang]['mem']['uss'][t] / 1024 / 1024 for t in x]
        ax.plot(x, mem_y, color=color, linewidth=1)

    ax.set_ylabel('Memória (mb)')
    ax.set_ylim(bottom=0)
    ax.tick_params(axis='y')
    plot_ticks(ax, act_data, xmax)
    ax.legend([*langs, 'Inscrições por conexão'])

elif which == 'cpu':
    ax.set_title(f'Comparação do uso de CPU (throughput)')

    legend = []
    for lang in langs:
        color = colors[lang]
        cpu_usr_y = [dfs[lang]['cpu']['user'][t] for t in x]
        cpu_sys_y = [dfs[lang]['cpu']['system'][t] for t in x]
        ax.plot(x, cpu_usr_y, color=color, linewidth=1, linestyle='--')
        ax.plot(x, cpu_sys_y, color=color, linewidth=1, linestyle=':')
        legend.append(f'{lang}, usuário')
        legend.append(f'{lang}, sistema')

    ax.set_ylabel('CPU (%)')
    ax.set_ylim(bottom=0)
    ax.tick_params(axis='y')
    plot_ticks(ax, act_data, xmax)
    legend.append('inscrições por conexão')
    ax.legend(legend)

ax.set_xlabel('Segundos após início do teste')

    
fig.tight_layout()
plt.savefig(f'graphs/throughput_{which}_comparison.png', dpi=172)
plt.close()
# plt.show()