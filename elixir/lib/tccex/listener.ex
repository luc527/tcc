defmodule Tccex.Listener do
  require Logger
  alias Tccex.{Client, Receiver}
  use Task, restart: :permanent

  @send_timeout_ms 10_000

  def start_link(arg) do
    Task.start_link(fn -> run(arg) end)
  end

  defp run({ip, port}) do
    opts = [
      nodelay: true,
      active: false,
      mode: :binary,
      ip: ip,
      send_timeout: @send_timeout_ms,
      send_timeout_close: true
    ]

    {:ok, lsock} = :gen_tcp.listen(port, opts)
    {:ok, port} = :inet.port(lsock)
    Logger.info("listening on #{inspect(ip)}:#{port}")
    accept_loop(lsock)
  end

  defp accept_loop(lsock) do
    {:ok, sock} = :gen_tcp.accept(lsock)
    {:ok, client_pid} = start_client(sock)
    {:ok, _receiver_pid} = start_receiver(sock, client_pid)
    :ok = :gen_tcp.controlling_process(sock, client_pid)
    accept_loop(lsock)
  end

  defp start_client(sock) do
    DynamicSupervisor.start_child(Client.Supervisor, {Client, sock})
  end

  defp start_receiver(sock, client_pid) do
    DynamicSupervisor.start_child(Receiver.Supervisor, {Receiver, {sock, client_pid}})
  end
end
