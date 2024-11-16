defmodule Tccex.Listener do
  use Task, restart: :permanent

  require Logger

  alias Tccex.Client

  @send_timeout_ms 10_000

  def start_link(arg) do
    Task.start_link(fn -> run(arg) end)
  end

  defp run({ip, port}) do
    backlog =
      case File.read("/proc/sys/net/core/somaxconn") do
        {:ok, binary} ->
          binary |> String.trim() |> String.to_integer()
        _ ->
          128
      end

    backlog = max(128, backlog)
    Logger.info("backlog: #{backlog}")

    opts = [
      nodelay: true,
      active: false,
      packet: 2,
      mode: :binary,
      ip: ip,
      backlog: backlog,
      send_timeout: @send_timeout_ms,
      send_timeout_close: true
    ]

    with {:ok, lsock} <- :gen_tcp.listen(port, opts),
         {:ok, port} <- :inet.port(lsock)
    do
      Logger.info("listening on #{format_ip(ip)}:#{port}")
      accept_loop(lsock)
    else
      error ->
        Logger.warning("listener failed to start: #{inspect(error)}")
    end
  end

  defp format_ip({a, b, c, d}), do: :io_lib.format("~B.~B.~B.~B", [a, b, c, d])

  defp format_ip({a, b, c, d, e, f, g, h}) do
    :io_lib.format(
      "~.16B" |> List.duplicate(8) |> Enum.join(":"),
      [a, b, c, d, e, f, g, h]
    )
  end

  defp accept_loop(lsock) do
    {:ok, sock} = :gen_tcp.accept(lsock)
    {:ok, client_pid} = start_client(sock)
    :ok = :gen_tcp.controlling_process(sock, client_pid)
    accept_loop(lsock)
  end

  defp start_client(sock) do
    DynamicSupervisor.start_child(Client.Supervisor, {Client, sock})
  end
end
