module Build.Shortcuts exposing (handleDelivery, keyboardHelp)

import Build.Header.Models exposing (HistoryItem)
import Build.Models exposing (ShortcutsModel)
import Concourse.BuildStatus
import EffectTransformer exposing (ET)
import Html exposing (Html)
import Html.Attributes exposing (class, classList)
import Keyboard
import Maybe.Extra
import Message.Effects exposing (Effect(..))
import Message.Message exposing (DomID(..), Message(..))
import Message.ScrollDirection exposing (ScrollDirection(..))
import Message.Subscription exposing (Delivery(..))
import Routes


bodyId : String
bodyId =
    "build-body"


keyboardHelp : Bool -> Html Message
keyboardHelp showHelp =
    let
        shortcuts =
            [ { keys = [ "h", "l" ], description = "previous/next build" }
            , { keys = [ "j", "k" ], description = "scroll down/up" }
            , { keys = [ "T" ], description = "trigger a new build" }
            , { keys = [ "R" ], description = "rerun the current build" }
            , { keys = [ "A" ], description = "abort build" }
            , { keys = [ "gg" ], description = "scroll to the top" }
            , { keys = [ "G" ], description = "scroll to the bottom" }
            , { keys = [ "?" ], description = "hide/show help" }
            ]

        keySpan key =
            Html.span [ class "key" ] [ Html.text key ]

        helpLine shortcut =
            Html.div
                [ class "help-line" ]
                [ Html.div [ class "keys" ] (List.map keySpan shortcut.keys)
                , Html.text shortcut.description
                ]
    in
    Html.div
        [ classList
            [ ( "keyboard-help", True )
            , ( "hidden", not showHelp )
            ]
        ]
        (Html.div [ class "help-title" ] [ Html.text "keyboard shortcuts" ]
            :: List.map helpLine shortcuts
        )


historyItem : ShortcutsModel r -> HistoryItem
historyItem model =
    { id = model.id
    , name = model.name
    , status = model.status
    , duration = model.duration
    }


prevHistoryItem : List HistoryItem -> HistoryItem -> Maybe HistoryItem
prevHistoryItem builds b =
    case builds of
        first :: second :: rest ->
            if first == b then
                Just second

            else
                prevHistoryItem (second :: rest) b

        _ ->
            Nothing


nextHistoryItem : List HistoryItem -> HistoryItem -> Maybe HistoryItem
nextHistoryItem builds b =
    case builds of
        first :: second :: rest ->
            if second == b then
                Just first

            else
                nextHistoryItem (second :: rest) b

        _ ->
            Nothing


handleDelivery : Delivery -> ET (ShortcutsModel r)
handleDelivery delivery ( model, effects ) =
    case delivery of
        KeyDown keyEvent ->
            handleKeyPressed keyEvent ( model, effects )

        KeyUp keyEvent ->
            case keyEvent.code of
                Keyboard.T ->
                    ( { model | isTriggerBuildKeyDown = False }, effects )

                _ ->
                    ( model, effects )

        _ ->
            ( model, effects )


handleKeyPressed : Keyboard.KeyEvent -> ET (ShortcutsModel r)
handleKeyPressed keyEvent ( model, effects ) =
    let
        newModel =
            case ( model.previousKeyPress, keyEvent.shiftKey, keyEvent.code ) of
                ( Nothing, False, Keyboard.G ) ->
                    { model | previousKeyPress = Just keyEvent }

                _ ->
                    { model | previousKeyPress = Nothing }
    in
    if Keyboard.hasControlModifier keyEvent then
        ( newModel, effects )

    else
        case ( keyEvent.code, keyEvent.shiftKey ) of
            ( Keyboard.J, False ) ->
                ( newModel, [ Scroll Down bodyId ] )

            ( Keyboard.K, False ) ->
                ( newModel, [ Scroll Up bodyId ] )

            ( Keyboard.G, True ) ->
                ( { newModel | autoScroll = True }, [ Scroll ToBottom bodyId ] )

            ( Keyboard.G, False ) ->
                if
                    (model.previousKeyPress |> Maybe.map .code)
                        == Just Keyboard.G
                then
                    ( { newModel | autoScroll = False }, [ Scroll ToTop bodyId ] )

                else
                    ( newModel, effects )

            ( Keyboard.Slash, True ) ->
                ( { newModel | showHelp = not newModel.showHelp }, effects )

            ( Keyboard.H, False ) ->
                case nextHistoryItem model.history (historyItem model) of
                    Just item ->
                        ( newModel
                        , effects
                            ++ [ NavigateTo <|
                                    Routes.toString <|
                                        Routes.buildRoute
                                            item.id
                                            item.name
                                            newModel.job
                               ]
                        )

                    Nothing ->
                        ( newModel, effects )

            ( Keyboard.L, False ) ->
                case prevHistoryItem newModel.history (historyItem newModel) of
                    Just item ->
                        ( newModel
                        , effects
                            ++ [ NavigateTo <|
                                    Routes.toString <|
                                        Routes.buildRoute
                                            item.id
                                            item.name
                                            newModel.job
                               ]
                        )

                    Nothing ->
                        ( newModel, effects )

            ( Keyboard.T, True ) ->
                if not newModel.isTriggerBuildKeyDown then
                    (newModel.job
                        |> Maybe.map (DoTriggerBuild >> (::) >> Tuple.mapSecond)
                        |> Maybe.withDefault identity
                    )
                        ( { newModel | isTriggerBuildKeyDown = True }, effects )

                else
                    ( newModel, effects )

            ( Keyboard.R, True ) ->
                ( newModel
                , effects
                    ++ (if Concourse.BuildStatus.isRunning newModel.status then
                            []

                        else
                            newModel.job
                                |> Maybe.map
                                    (\j ->
                                        RerunJobBuild
                                            { pipelineId = j.pipelineId
                                            , jobName = j.jobName
                                            , buildName = newModel.name
                                            }
                                    )
                                |> Maybe.Extra.toList
                       )
                )

            ( Keyboard.A, True ) ->
                if Just (historyItem newModel) == List.head newModel.history then
                    ( newModel, DoAbortBuild newModel.id :: effects )

                else
                    ( newModel, effects )

            _ ->
                ( newModel, effects )
