defmodule Tcc.Rooms.Supervisor do
  use Supervisor

  @rooms_partitions System.schedulers_online()

  def start_link(init_arg) do
    Supervisor.start_link(__MODULE__, init_arg, name: __MODULE__)
  end

  def init(_) do
    children = [
      {Registry, name: Tcc.Rooms.Partition.Registry, keys: :unique},
      {DynamicSupervisor, name: Tcc.Rooms.Partition.Supervisor, strategy: :one_for_one},
      {Tcc.Rooms, @rooms_partitions}
    ]
    opts = [
      name: __MODULE__,
      strategy: :rest_for_one,
    ]
    Supervisor.init(children, opts)
  end
end
