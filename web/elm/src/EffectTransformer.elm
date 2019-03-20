module EffectTransformer exposing (ET)

import Message.Effects exposing (Effect)


type alias ET a =
    ( a, List Effect ) -> ( a, List Effect )
