defmodule Tccex.Application do
  use Application
  require Logger


  # TODO: put this in a config
  @ip :loopback
  @port 17302

  @impl true
  def start(_type, _args) do
    children = [
      tcp_supervisor_child_spec(@ip, @port),
    ]

    Logger.info "starting application"
    Supervisor.start_link(children, name: Tccex.Supervisor, strategy: :one_for_one)
  end

  def tcp_supervisor_child_spec(ip, port) do
    children = [
      {DynamicSupervisor, name: Tccex.Client.Supervisor, strategy: :one_for_one},
      {DynamicSupervisor, name: Tccex.Tcp.Receiver.Supervisor, strategy: :one_for_one},
      {Tccex.Tcp.Listener, {ip, port}},
    ]
    name = Tccex.Tcp.Supervisor
    opts = [name: name, strategy: :rest_for_one]
    %{
      id: name,
      start: {Supervisor, :start_link, [children, opts]},
      type: :supervisor,
    }
  end
end
