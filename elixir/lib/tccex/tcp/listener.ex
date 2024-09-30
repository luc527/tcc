defmodule Tccex.Tcp.Listener do
  use Task, restart: :permanent
  alias Tccex.{Client, Tcp}
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
    Logger.info("listening at port #{port}")

    try do
      accept_loop(lsock)
    after
      :gen_tcp.close(lsock)
    end
  end

  defp accept_loop(lsock) do
    {:ok, sock} = :gen_tcp.accept(lsock)
    Logger.info("accepted sock: #{inspect(sock)}")
    {:ok, client_id} = start_client(sock)
    {:ok, receiver_pid} = start_receiver(sock, client_id)
    :ok = :gen_tcp.controlling_process(sock, receiver_pid)
    accept_loop(lsock)
  end

  defp start_client(sock) do
    # TODO: doesn't feel right for this to be in the tcp listener
    # maybe should be in an actual Client.Supervisor module, in a .start_child(sock) function?
    id = :erlang.unique_integer([:monotonic, :positive])

    with {:ok, _} <- DynamicSupervisor.start_child(Client.Supervisor, {Client, {sock, id}}) do
      {:ok, id}
    end
  end

  defp start_receiver(sock, client_id) do
    DynamicSupervisor.start_child(Tcp.Receiver.Supervisor, {Tcp.Receiver, {sock, client_id}})
  end
end
