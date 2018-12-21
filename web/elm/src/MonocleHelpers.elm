module MonocleHelpers exposing ((<|=), (=|>), (>>=), modifyWithEffect)

import Monocle.Lens
import Monocle.Optional


(=|>) :
    Monocle.Optional.Optional a b
    -> Monocle.Lens.Lens b c
    -> Monocle.Optional.Optional a c
(=|>) =
    Monocle.Optional.composeLens


(<|=) :
    Monocle.Lens.Lens a b
    -> Monocle.Optional.Optional b c
    -> Monocle.Optional.Optional a c
(<|=) =
    Monocle.Optional.compose << Monocle.Optional.fromLens



-- bind, like in Haskell


(>>=) :
    Monocle.Optional.Optional a b
    -> (b -> Monocle.Optional.Optional a c)
    -> Monocle.Optional.Optional a c
(>>=) opt f =
    { getOption =
        \a ->
            opt.getOption a
                |> Maybe.andThen (\b -> (f b).getOption a)
    , set =
        \c a ->
            opt.getOption a
                |> Maybe.map (\b -> (f b).set c a)
                |> Maybe.withDefault a
    }


modifyWithEffect :
    Monocle.Optional.Optional a b
    -> (b -> ( b, List c ))
    -> a
    -> ( a, List c )
modifyWithEffect l f m =
    l.getOption m
        |> Maybe.map f
        |> Maybe.map (Tuple.mapFirst (flip l.set m))
        |> Maybe.withDefault ( m, [] )
