defmodule Tcc.Application do
  use Application

  # TODO: config again
  @ip :loopback
  @port 0

  @rooms_partitions System.schedulers_online()

  @impl true
  def start(_type, _args) do
    children = [
      Tcc.Clients.Tables,
      {Registry, name: Tcc.Rooms.Partition.Registry, keys: :unique},
      {DynamicSupervisor, name: Tcc.Rooms.Partition.Supervisor, strategy: :one_for_one},
      {Tcc.Rooms, @rooms_partitions},
      Tcc.Clients,
      {DynamicSupervisor, name: Tcc.Client.Supervisor, strategy: :one_for_one},
      {Tcc.Tcp.Listener, {@ip, @port}}
    ]

    opts = [name: Tcc.Supervisor, strategy: :rest_for_one]
    Supervisor.start_link(children, opts)
  end
end
