defmodule Tcc.Tcp.Listener do
  require Logger
  alias Tcc.{Tcp, Client}
  use Task

  @send_timeout 10_000

  def start_link({ip, port}) do
    Task.start_link(fn -> run(ip, port) end)
  end

  def run(ip, port) do
    opts = [
      ip: ip,
      nodelay: true,
      mode: :binary,
      active: false,
      send_timeout: @send_timeout,
      send_timeout_close: true
    ]

    {:ok, lsock} = :gen_tcp.listen(port, opts)
    {:ok, port} = :inet.port(lsock)
    Logger.info("listening at 127.0.0.1:#{port}")

    try do
      accept_loop(lsock)
    after
      :gen_tcp.close(lsock)
    end
  end

  defp accept_loop(lsock) do
    {:ok, sock} = :gen_tcp.accept(lsock)
    {:ok, conn_pid} = Tcp.Connection.start(sock)
    :ok = :gen_tcp.controlling_process(sock, conn_pid)
    {:ok, client_id} = start_client(conn_pid)
    Tcp.Connection.start_receiver(conn_pid, client_id)
    accept_loop(lsock)
  end

  defp start_client(conn_pid) do
    {:ok, client_pid} = DynamicSupervisor.start_child(Client.Supervisor, {Client, {nil, conn_pid}})
    client_id = Tcc.Client.id(client_pid)
    {:ok, client_id}
  end
end
