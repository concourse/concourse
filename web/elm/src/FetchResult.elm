module FetchResult exposing
    ( FetchResult(..)
    , changedFrom
    , map
    , withDefault
    )


type FetchResult a
    = None
    | Cached a
    | Fetched a


value : FetchResult a -> Maybe a
value result =
    case result of
        None ->
            Nothing

        Cached cachedValue ->
            Just cachedValue

        Fetched fetchedValue ->
            Just fetchedValue


withDefault : a -> FetchResult a -> a
withDefault default =
    value >> Maybe.withDefault default


map : (a -> b) -> FetchResult a -> FetchResult b
map fn result =
    case result of
        None ->
            None

        Cached cachedValue ->
            Cached (fn cachedValue)

        Fetched fetchedValue ->
            Fetched (fn fetchedValue)


changedFrom : FetchResult a -> FetchResult a -> Bool
changedFrom oldResult newResult =
    case ( oldResult, newResult ) of
        ( Fetched old, Fetched new ) ->
            old /= new

        ( Cached old, Cached new ) ->
            old /= new

        ( Cached old, Fetched new ) ->
            old /= new

        ( None, Cached _ ) ->
            True

        ( None, Fetched _ ) ->
            True

        _ ->
            False
