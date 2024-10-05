defmodule Tccex.Tcp.Listener do
  use Task, restart: :permanent
  alias Tccex.{Tcp, Client}
  require Logger

  def start_link({ip, port}) do
    Task.start_link(fn -> listen(ip, port) end)
  end

  defp listen(ip, port) do
    opts = [
      ip: ip,
      active: false,
      mode: :binary,
      nodelay: true
    ]

    {:ok, lsock} = :gen_tcp.listen(port, opts)
    {:ok, port} = :inet.port(lsock)
    Logger.info("listener: listening at port #{port}")

    try do
      accept_loop(lsock)
    after
      :gen_tcp.close(lsock)
    end
  end

  defp accept_loop(lsock) do
    {:ok, sock} = :gen_tcp.accept(lsock)
    {:ok, conn_pid} = Tcp.Connection.start(sock)
    {:ok, client_id} = start_client(conn_pid)
    Tcp.Connection.start_receiver(conn_pid, client_id)
    accept_loop(lsock)
  end

  defp start_client(conn_pid) do
    client_id = :erlang.unique_integer([:monotonic, :positive])
    spec = {Client, {client_id, conn_pid}}

    with {:ok, _} <- DynamicSupervisor.start_child(Client.Supervisor, spec) do
      {:ok, client_id}
    end
  end
end
