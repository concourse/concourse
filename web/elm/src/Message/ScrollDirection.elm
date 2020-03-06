module Message.ScrollDirection exposing (ScrollDirection(..))


type ScrollDirection
    = ToTop
    | Down
    | Up
    | ToBottom
    | Sideways Float
    | ToId String
