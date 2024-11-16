defmodule Tccex.Application do
  use Application
  require Logger

  @impl true
  def start(_type, _args) do
    ip = System.get_env("HOST", "127.0.0.1")
    port = System.get_env("PORT", "0")

    {:ok, ip} = ip |> String.to_charlist |> :inet.parse_address
    port = port |> String.to_integer

    children = [
      {Registry,
       keys: :duplicate, partitions: System.schedulers_online(), name: Tccex.Topic.Registry},
      {DynamicSupervisor, strategy: :one_for_one, name: Tccex.Client.Supervisor},
      {Tccex.Listener, {ip, port}}
    ]

    opts = [strategy: :rest_for_one, name: Tccex.Supervisor]
    Supervisor.start_link(children, opts)
  end
end
