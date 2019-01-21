port module Main exposing (main)

import Effects
import Layout
import Navigation


main : Program Layout.Flags Layout.Model Layout.Msg
main =
    Navigation.programWithFlags Layout.locationMsg
        { init = \flags -> Layout.init flags >> Tuple.mapSecond effectsToCmd
        , update = \msg -> Layout.update msg >> Tuple.mapSecond effectsToCmd
        , view = Layout.view
        , subscriptions = Layout.subscriptions
        }


effectsToCmd : List ( Effects.LayoutDispatch, Effects.Effect ) -> Cmd Layout.Msg
effectsToCmd =
    List.map effectToCmd >> Cmd.batch


effectToCmd : ( Effects.LayoutDispatch, Effects.Effect ) -> Cmd Layout.Msg
effectToCmd ( disp, eff ) =
    Effects.runEffect eff |> Cmd.map (Layout.Callback disp)
