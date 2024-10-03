defmodule Tccex.Client do
  use GenServer
  require Logger
  require Tccex.Message
  alias Tccex.{Message, Client}
  import Tccex.Utils

  @max_rooms 256

  defstruct sock: nil,
            rooms: %{},
            id: 0

  @type t :: %__MODULE__{
          sock: port,
          rooms: %{integer => String.t()},
          id: integer
        }

  @ping_interval 20_000

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
    Process.send_after(self(), :send_ping, @ping_interval)
    Registry.register(Client.Registry, id, nil)
    {:ok, new(sock: sock, id: id)}
  end

  @impl true
  def handle_info(:send_ping, %{sock: sock} = state) do
    packet = Message.encode(:ping)

    with :ok <- :gen_tcp.send(sock, packet) do
      Process.send_after(self(), :send_ping, @ping_interval)
      {:noreply, state}
    else
      {:error, reason} ->
        {:stop, stop_reason(reason), state}
    end
  end

  @impl true
  def handle_call({:recv, message}, _from, state) do
    handle_incoming(message, state)
  end

  # mock
  defp join_room(id, room, name) do
    Logger.info("client #{id} joining room #{room} with name #{name}")
    Room.Hub.join_room(id, room, name)
 end

  # mock
  defp exit_room(id, room) do
    Logger.info("client #{id} exiting room #{room}")
    Room.Hub.exit_room(id, room)
  end

  # mock
  defp talk_in_room(id, room, text) do
    Logger.info("#{id} talking in room #{room}: \"#{text}\"")
    Room.Hub.talk_in_room(id, room, text)
  end

  defp check_not_in_room(rooms, room) do
    check(not Map.has_key?(rooms, room) , :joined)
  end

  defp check_in_room(rooms, room) do
    check(Map.has_key?(rooms, room) , :bad_room)
  end

  defp check_name_is_valid(name) do
    n = byte_size(name)
    ok = n > 0 and n <= 24 and not String.contains?(name, "\n")
    check(ok , :bad_name)
  end

  defp check_below_room_limit(rooms) do
    check(map_size(rooms) < @max_rooms, :room_limit)
  end

  defp check_message_text(text) do
    n = byte_size(text)
    check(n > 0 and n <= 2048, :bad_message)
  end

  defp send_error_or_stop({:error, e} = error, mtype, state) do
    if Message.is_error_for(e, mtype) do
      send_or_stop({:prob, e}, state, :ok)
    else
      send_or_stop({:prob, {:transient, mtype}}, state, error)
    end
  end

  defp handle_incoming(:pong, state) do
    {:reply, :ok, state}
  end

  defp handle_incoming({:join, room, name}, %{rooms: rooms, id: id} = state) do
    with :ok <- check_not_in_room(rooms, room),
         :ok <- check_below_room_limit(rooms),
         :ok <- check_name_is_valid(name),
         :ok <- join_room(id, room, name) do
      state = put_in(state.rooms[room], name)
      {:reply, :ok, state}
    else
      error ->
        send_error_or_stop(error, :join, state)
    end
  end

  defp handle_incoming({:exit, room}, %{rooms: rooms, id: id} = state) do
    with :ok <- check_in_room(rooms, room),
         :ok <- exit_room(id, room) do
      state = update_in(state.rooms, &Map.delete(&1, room))
      {:reply, :ok, state}
    else
      error ->
        send_error_or_stop(error, :exit, state)
    end
  end

  defp handle_incoming({:talk, room, text}, %{rooms: rooms, id: id} = state) do
    with :ok <- check_in_room(rooms, room),
         :ok <- check_message_text(text),
         :ok <- talk_in_room(id, room, text) do
      {:reply, :ok, state}
    else
      error -> send_error_or_stop(error, :talk, state)
    end
  end

  # TODO: test
  defp handle_incoming(:lsro, %{rooms: rooms} = state) do
    send_or_stop({:rols, rooms}, state)
  end

  defp stop_reason(:closed), do: :normal
  defp stop_reason(reason), do: reason

  defp send_or_stop(message, %{sock: sock} = state, reply \\ :ok) do
    packet = Message.encode(message)

    with :ok <- :gen_tcp.send(sock, packet) do
      {:reply, reply, state}
    else
      {:error, reason} ->
        {:stop, stop_reason(reason), state}
    end
  end

  def recv(id, message) do
    GenServer.call(via(id), {:recv, message})
  end
end
