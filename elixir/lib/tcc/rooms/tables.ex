defmodule Tcc.Rooms.Tables do
  @type t :: %{
    room_client: reference,
    room_name: reference,
  }

  def new() do
    %{
      room_client: :ets.new(nil, [:ordered_set, :public]),
      room_name: :ets.new(nil, [:ordered_set, :public]),
    }
  end

  def has_name?(%{room_name: rn}, room, name) do
    [] != :ets.lookup(rn, {room, name})
  end

  def has_client?(%{room_client: rc}, room, client) do
    [] != :ets.lookup(rc, {room, client})
  end

  def insert(%{room_client: rc, room_name: rn}, room, name, client) do
    true = :ets.insert_new(rc, {{room, client}})
    true = :ets.insert_new(rn, {{room, name}})
  end

  def delete(%{room_client: rc, room_name: rn}, room, name, client) do
    true = :ets.delete(rc, {room, client})
    true = :ets.delete(rn, {room, name})
  end

  def client_count(%{room_client: table}, room) do
    match = {{room, :_}}
    :ets.select_count(table, [{match, [], [true]}])
  end

  def clients(%{room_client: table}, room) do
    :ets.match(table, {{room, :"$1"}})
    |> Enum.map(fn [id] -> id end)
  end


end
