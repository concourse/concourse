module FetchResult exposing
    ( FetchResult(..)
    , gotCache
    , gotResult
    , map
    , value
    , withDefault
    )


type FetchResult a
    = None
    | Cached a
    | Fetched a


gotCache : a -> FetchResult a -> FetchResult a
gotCache cachedValue result =
    case result of
        Fetched _ ->
            result

        _ ->
            Cached cachedValue


gotResult : a -> FetchResult a -> FetchResult a
gotResult fetchedValue _ =
    Fetched fetchedValue


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
