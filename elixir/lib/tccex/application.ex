defmodule Tccex.Application do
  use Application
  require Logger

  @impl true
  def start(_type, _args) do
    ip = Application.fetch_env!(:tccex, :ip)
    port = Application.fetch_env!(:tccex, :port)

    # TODO: look into :listeners Registry option, could use for the jned and exed messages

    children = [
      {Registry, name: Tccex.Client.Registry, keys: :unique},
      {DynamicSupervisor, name: Tccex.Client.Supervisor, strategy: :one_for_one},
      {DynamicSupervisor, name: Tccex.Tcp.Receiver.Supervisor, strategy: :one_for_one},
      {Tccex.Tcp.Listener, {ip, port}}
    ]

    Supervisor.start_link(children, name: Tccex.Supervisor, strategy: :one_for_one)
  end
end
