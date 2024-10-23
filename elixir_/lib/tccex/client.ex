defmodule Tccex.Client do
  require Logger
  alias Tccex.{Message, Topic}
  use GenServer, restart: :transient

  def start_link(init_arg) do
    GenServer.start_link(__MODULE__, init_arg)
  end

  def init(sock) do
    {:ok, sock}
  end

  def handle_continue({:response, msg}, sock) do
    send_back(msg, sock)
  end

  def handle_cast({:pub, _topic, _payload} = msg, sock) do
    send_back(msg, sock)
  end

  def handle_call({:conn_msg, msg}, _from, sock) do
    case handle_conn_msg(msg) do
      {:response, _} = resp -> {:reply, :ok, sock, {:continue, resp}}
      _ -> {:reply, :ok, sock}
    end
  end

  defp handle_conn_msg(:ping) do
    Logger.info("received ping, responding ping")
    {:response, :ping}
  end

  defp handle_conn_msg({:sub, topic}) do
    Logger.info("received sub #{topic}, subscribing")
    Registry.register(Topic.Registry, topic, nil)
    {:response, {:subbed, topic, true}}
  end

  defp handle_conn_msg({:unsub, topic}) do
    Logger.info("received unsub #{topic}, unsubscribing")
    Registry.unregister(Topic.Registry, topic)
    {:response, {:subbed, topic, false}}
  end

  defp handle_conn_msg({:pub, topic, payload}) do
    Logger.info("received pub #{topic};#{payload}, publishing")

    Registry.dispatch(Topic.Registry, topic, fn clients ->
      Logger.info("publishing to pids: #{inspect(clients)}")
      Enum.each(clients, fn {pid, _value} ->
        GenServer.cast(pid, {:pub, topic, payload})
      end)
    end)

    :ok
  end

  defp send_back(msg, sock) do
    Logger.info("sending msg #{inspect(msg)}")
    packet = Message.encode(msg)

    with :ok <- :gen_tcp.send(sock, packet) do
      {:noreply, sock}
    else
      {:error, reason} ->
        Logger.warning("client: send failed with reason #{inspect(reason)}, stopping")
        {:stop, :normal, sock}
    end
  end

  def ping(pid), do: GenServer.call(pid, {:conn_msg, :ping})
  def sub(pid, topic), do: GenServer.call(pid, {:conn_msg, {:sub, topic}})
  def pub(pid, topic, payload), do: GenServer.call(pid, {:conn_msg, {:pub, topic, payload}})
  def unsub(pid, topic), do: GenServer.call(pid, {:conn_msg, {:unsub, topic}})
end
