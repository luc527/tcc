defmodule Tccex.Application do
  use Application

  # TODO: env
  @ip {127,0,0,1}
  # ephemeral
  @port 0

  @impl true
  def start(_type, _args) do
    children = [
      {Registry,
       keys: :duplicate, partitions: System.schedulers_online(), name: Tccex.Topic.Registry},
      {DynamicSupervisor, strategy: :one_for_one, name: Tccex.Client.Supervisor},
      {Tccex.Listener, {@ip, @port}}
    ]

    opts = [strategy: :rest_for_one, name: Tccex.Supervisor]
    Supervisor.start_link(children, opts)
  end
end
