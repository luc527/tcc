defmodule Tcc.Application do
  use Application

  # TODO: config again
  @ip :loopback
  @port 0

  @impl true
  def start(_type, _args) do
    children = [
      Tcc.Rooms.Supervisor,
      Tcc.Clients.Supervisor,
      {DynamicSupervisor, name: Tcc.Client.Supervisor, strategy: :one_for_one},
      {Tcc.Tcp.Listener, {@ip, @port}}
    ]

    Supervisor.start_link(children, name: Tcc.Supervisor, strategy: :rest_for_one)
  end
end
