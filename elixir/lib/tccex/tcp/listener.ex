defmodule Tccex.Tcp.Listener do
  use Task, restart: :permanent
  require Logger

  def start_link({ip, port}) do
    Logger.info "starting listener, ip: #{ip}, port: #{port}"
    Task.start_link(fn -> listen(ip, port) end)
  end

  defp listen(ip, port) do
    opts = [
      ip: ip,
      active: false,
      mode: :binary,
      nodelay: true,
    ]
    {:ok, lsock} = :gen_tcp.listen(port, opts)
    try do
      accept_loop(lsock)
    after
      :gen_tcp.close(lsock)
    end
  end

  defp accept_loop(lsock) do
    Logger.info "waiting for connections..."
    {:ok, sock} = :gen_tcp.accept(lsock)
    Logger.info "accepted, sock: #{inspect sock}"
    {:ok, client_pid} = start_client(sock)
    {:ok, receiver_pid} = start_receiver(sock, client_pid)
    :ok = :gen_tcp.controlling_process(sock, receiver_pid)
    accept_loop(lsock)
  end

  defp start_client(sock) do
    DynamicSupervisor.start_child(Tccex.Client.Supervisor, {Tccex.Client, sock})
  end

  defp start_receiver(sock, client_pid) do
    DynamicSupervisor.start_child(Tccex.Tcp.Receiver.Supervisor, {Tccex.Tcp.Receiver, {sock, client_pid}})
  end
end
