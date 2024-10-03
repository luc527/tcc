defmodule Tcc.Application do
  use Application

  @rooms_partitions System.schedulers_online()

  @impl true
  def start(_type, _args) do
    children = [
      Tcc.Clients.Tables,
      Tcc.Clients,
      {Registry, name: Tcc.Rooms.Partition.Registry, keys: :unique},
      {DynamicSupervisor, name: Tcc.Rooms.Partition.Supervisor, strategy: :one_for_one},
      {Tcc.Rooms, @rooms_partitions}
    ]

    opts = [name: Tcc.Supervisor, strategy: :rest_for_one]
    Supervisor.start_link(children, opts)
  end
end
