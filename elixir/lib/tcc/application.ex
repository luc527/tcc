defmodule Tcc.Application do
  use Application

  # TODO: config again
  @ip :loopback
  @port 0

  # Tcc.Clients relies on the fact that the number of partitions doesn't change after system startup
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

    Supervisor.start_link(children, name: Tcc.Supervisor, strategy: :rest_for_one)
  end
end
