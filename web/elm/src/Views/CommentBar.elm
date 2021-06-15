module Views.CommentBar exposing
    ( Model
    , State(..)
    , Style
    , ViewState
    , commentSetCallback
    , defaultStyle
    , getContent
    , getTextAreaID
    , resetState
    , setCachedContent
    , update
    , view
    )

import Assets
import Colors
import HoverState exposing (HoverState)
import Html exposing (Attribute, Html)
import Html.Attributes exposing (id, readonly, style, value)
import Html.Events exposing (onBlur, onClick, onFocus, onInput, onMouseEnter, onMouseLeave)
import Http
import Message.Effects exposing (Effect(..), toHtmlID)
import Message.Message as Message exposing (CommentBarButtonKind(..), DomID)
import Views.Icon as Icon
import Views.Spinner as Spinner
import Views.Styles as Styles


type alias Model =
    { id : Message.DomID
    , state : State
    , style : Style
    }


type alias ViewState =
    { hover : HoverState
    , editable : Bool
    }


type alias UpdateState =
    { saveComment : String -> Effect
    }


type alias Style =
    { viewBackgroundColor : String
    , editBackgroundColor : String
    }


type alias CachedState =
    { content : String
    , cached : String
    }


type State
    = Viewing String
    | Editing CachedState
    | Saving CachedState


defaultStyle : Style
defaultStyle =
    { viewBackgroundColor = "transparent"
    , editBackgroundColor = "rgba(255, 255, 255, 0.1)"
    }


getContent : Model -> String
getContent model =
    case model.state of
        Viewing content ->
            content

        Editing { content } ->
            content

        Saving { content } ->
            content


getTextAreaID : Model -> DomID
getTextAreaID model =
    Message.CommentBar model.id


setCachedContent : String -> Model -> Model
setCachedContent content model =
    { model
        | state =
            case model.state of
                Viewing _ ->
                    Viewing content

                Editing state ->
                    Editing { state | cached = content }

                Saving state ->
                    Saving { state | cached = content }
    }


resetState : Model -> ( Model, List Effect )
resetState model =
    let
        content =
            getContent model

        viewOrEditState =
            if String.isEmpty content then
                Editing { content = content, cached = content }

            else
                Viewing content

        updatedState =
            case model.state of
                Viewing _ ->
                    viewOrEditState

                Editing _ ->
                    viewOrEditState

                -- Don't change away from "Saving" state, wait for callback instead
                Saving state ->
                    Saving state
    in
    ( { model | state = updatedState }
    , SyncTextareaHeight (getTextAreaID model)
        :: (case updatedState of
                Editing _ ->
                    [ Focus (toHtmlID (getTextAreaID model)) ]

                _ ->
                    []
           )
    )


commentTextArea : Model -> Html Message.Message
commentTextArea model =
    let
        isReadOnly =
            case model.state of
                Editing _ ->
                    False

                _ ->
                    True
    in
    Html.textarea
        ([ id (toHtmlID (getTextAreaID model))
         , value (getContent model)
         , onInput (Message.EditCommentBar model.id)
         , onFocus (Message.FocusCommentBar model.id)
         , onBlur (Message.BlurCommentBar model.id)
         , readonly isReadOnly
         , if isReadOnly then
            style "background-color" model.style.viewBackgroundColor

           else
            style "background-color" model.style.editBackgroundColor
         ]
            ++ Styles.commentBarTextArea
        )
        []


editButton : Model -> ViewState -> Html Message.Message
editButton model state =
    let
        htmlID =
            Message.CommentBarButton Message.Edit model.id
    in
    Icon.icon
        { sizePx = 16
        , image = Assets.PencilIcon
        }
        ([ id (toHtmlID htmlID)
         , onMouseEnter (Message.Hover (Just htmlID))
         , onMouseLeave (Message.Hover Nothing)
         , onClick (Message.Click htmlID)
         , if HoverState.isHovered htmlID state.hover then
            style "background-color" Colors.sectionHeader

           else
            style "background-color" Colors.pinTools
         ]
            ++ Styles.commentBarEditButton
        )


loadingButton : Html Message.Message
loadingButton =
    Html.div
        ([ style "background-color" "transparent"
         , style "color" Colors.buttonDisabledGrey
         ]
            ++ Styles.commentBarTextButton
        )
        [ Spinner.spinner { sizePx = 12, margin = "0" } ]


saveButton : Model -> ViewState -> Html Message.Message
saveButton model state =
    Html.button
        (let
            htmlID =
                Message.CommentBarButton Message.Save model.id
         in
         [ id (toHtmlID htmlID)
         , onMouseEnter (Message.Hover (Just htmlID))
         , onMouseLeave (Message.Hover Nothing)
         , onClick (Message.Click htmlID)
         , style "color" Colors.text
         , if HoverState.isHovered htmlID state.hover then
            style "background-color" Colors.frame

           else
            style "background-color" "transparent"
         ]
            ++ Styles.commentBarTextButton
        )
        [ Html.text "save" ]


view : Model -> ViewState -> List (Attribute Message.Message) -> Html Message.Message
view model state attrs =
    Html.div
        ([ style "display" "flex"
         , style "align-items" "flex-start"
         ]
            ++ Styles.commentBarWrapper
            ++ attrs
        )
        (Icon.icon
            { sizePx = 16
            , image = Assets.MessageIcon
            }
            [ style "margin" "10px"
            , style "flex-shrink" "0"
            , style "background-size" "contain"
            , style "background-origin" "content-box"
            ]
            :: (if state.editable then
                    [ commentTextArea model
                    , Html.div
                        [ style "width" "60px"
                        , style "display" "flex"
                        , style "justify-content" "flex-end"
                        ]
                        (case model.state of
                            Viewing _ ->
                                [ editButton model state ]

                            Editing _ ->
                                [ saveButton model state ]

                            Saving _ ->
                                [ loadingButton ]
                        )
                    ]

                else
                    [ Html.pre
                        Styles.commentBarText
                        [ Html.text (getContent model) ]
                    ]
               )
        )


commentSetCallback : ( String, Result Http.Error () ) -> Model -> Model
commentSetCallback ( content, result ) model =
    { model
        | state =
            case ( result, model.state ) of
                ( Ok (), Viewing _ ) ->
                    Viewing content

                ( Ok (), Editing state ) ->
                    Editing { state | cached = content }

                ( Ok (), Saving _ ) ->
                    Viewing content

                ( Err _, Saving state ) ->
                    Editing state

                ( _, state ) ->
                    state
    }


update : Model -> UpdateState -> Message.Message -> ( Model, List Effect )
update model updateState msg =
    case msg of
        Message.Click (Message.CommentBarButton kind id) ->
            if model.id == id then
                case kind of
                    Message.Edit ->
                        case model.state of
                            Viewing content ->
                                ( { model | state = Editing { content = content, cached = content } }
                                , [ Focus (toHtmlID (getTextAreaID model)) ]
                                )

                            _ ->
                                ( model, [] )

                    Message.Save ->
                        case model.state of
                            Editing state ->
                                if state.content /= state.cached then
                                    ( { model | state = Saving state }, [ updateState.saveComment state.content ] )

                                else
                                    ( { model | state = Viewing state.content }, [] )

                            _ ->
                                ( model, [] )

            else
                ( model, [] )

        Message.EditCommentBar id content ->
            if model.id == id then
                case model.state of
                    Editing state ->
                        ( { model | state = Editing { state | content = content } }
                        , [ SyncTextareaHeight (getTextAreaID model) ]
                        )

                    _ ->
                        ( model, [] )

            else
                ( model, [] )

        _ ->
            ( model, [] )
