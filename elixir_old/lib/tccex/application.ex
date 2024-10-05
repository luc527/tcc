defmodule Tccex.Application do
  use Application
  require Logger

  @impl true
  def start(_type, _args) do
    ip = Application.fetch_env!(:tccex, :ip)
    port = Application.fetch_env!(:tccex, :port)

    children = [
      hub_supervisor_spec(),
      {Registry, name: Tccex.Client.Registry, keys: :unique, listeners: [Tccex.Hub]},
      {DynamicSupervisor, name: Tccex.Client.Supervisor, strategy: :one_for_one},
      {Tccex.Tcp.Listener, {ip, port}}
    ]

    Supervisor.start_link(children, name: Tccex.Supervisor, strategy: :rest_for_one)
  end

  defp hub_supervisor_spec() do
    children = [
      {Registry, name: Tccex.Hub.Partition.Registry, keys: :unique},
      {DynamicSupervisor, name: Tccex.Hub.Partition.Supervisor, strategy: :one_for_one},
      {Tccex.Hub, partitions: System.schedulers_online()},
    ]
    name = Tccex.Hub.Supervisor
    opts = [
      name: name,
      strategy: :one_for_all,
    ]
    %{
      id: name,
      start: {Supervisor, :start_link, [children, opts]},
      type: :supervisor,
    }
  end
end
