defmodule Tccex.Client do
  use GenServer
  alias Tccex.{Message, Client}
  require Logger

  defstruct sock: nil,
            rooms: %{},
            id: 0

  @type t :: %__MODULE__{
          sock: port,
          rooms: %{integer => String.t()},
          id: integer
        }

  @pingInterval 20_000

  def new, do: new([])
  def new(fields), do: struct!(__MODULE__, fields)

  def start_link({sock, id}) do
    GenServer.start_link(__MODULE__, {sock, id}, name: via(id))
  end

  defp via(id) do
    {:via, Registry, {Client.Registry, id}}
  end

  @impl true
  def init({sock, id}) when is_port(sock) and is_integer(id) do
    Process.send_after(self(), :send_ping, @pingInterval)
    Registry.register(Client.Registry, id, nil)
    {:ok, new(sock: sock, id: id)}
  end

  @impl true
  def handle_info(:send_ping, %{sock: sock} = state) do
    packet = Message.encode(:ping)

    with :ok <- :gen_tcp.send(sock, packet) do
      Process.send_after(self(), :send_ping, @pingInterval)
      {:noreply, state}
    else
      {:error, reason} ->
        {:stop, exit_reason(reason), state}
    end
  end

  @impl true
  def handle_call({:recv, message}, _from, state) do
    handle_incoming(message, state)
  end

  defp handle_incoming(:pong, state), do: {:reply, :ok, state}

  # TODO: rest of them
  defp handle_incoming(msg, state) do
    {:reply, msg, state}
  end

  defp send_or_stop(message, %{sock: sock} = state) do
    packet = Message.encode(message)

    with :ok <- :gen_tcp.send(sock, packet) do
      {:reply, :ok, state}
    else
      {:error, reason} ->
        {:stop, exit_reason(reason), state}
    end
  end

  defp exit_reason(:closed), do: :normal
  defp exit_reason(reason), do: reason

  def recv(id, message) do
    GenServer.call(via(id), {:recv, message})
  end
end
