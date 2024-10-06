defmodule Tcc.Clients.Supervisor do
  use Supervisor

  @clients_partitions System.schedulers_online()

  def start_link(init_arg) do
    Supervisor.start_link(__MODULE__, init_arg, name: __MODULE__)
  end

  def init(_) do
    children = [
      {Registry, name: Tcc.Clients.Partition.Registry, keys: :unique},
      {DynamicSupervisor, name: Tcc.Clients.Partition.Supervisor, strategy: :one_for_one},
      {Tcc.Clients, @clients_partitions}
    ]
    opts = [
      name: __MODULE__,
      strategy: :rest_for_one,
    ]
    Supervisor.init(children, opts)
  end
end
