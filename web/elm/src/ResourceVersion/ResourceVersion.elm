module ResourceVersion.ResourceVersion exposing
    ( Flags
    , changeToResourceVersion
    , documentTitle
    , getUpdateMessage
    , handleCallback
    , handleDelivery
    , init
    , subscriptions
    , update
    , view
    , viewPinButton
    , viewVersionBody
    , viewVersionHeader
    )

import Application.Models exposing (Session)
import Concourse
import Concourse.BuildStatus
import DateFormat
import Dict
import Duration
import EffectTransformer exposing (ET)
import Html exposing (Html)
import Html.Attributes
    exposing
        ( attribute
        , class
        , href
        , id
        , placeholder
        , style
        , title
        , value
        )
import Html.Events
    exposing
        ( onBlur
        , onClick
        , onFocus
        , onInput
        , onMouseEnter
        , onMouseLeave
        , onMouseOut
        , onMouseOver
        )
import Http
import Login.Login as Login
import Maybe.Extra as ME
import Message.Callback exposing (Callback(..))
import Message.Effects exposing (Effect(..))
import Message.Message as Message
    exposing
        ( DomID(..)
        , Message(..)
        )
import Message.Subscription as Subscription
    exposing
        ( Delivery(..)
        , Interval(..)
        , Subscription(..)
        )
import Message.TopLevelMessage exposing (TopLevelMessage(..))
import Pinned exposing (ResourcePinState(..), VersionPinState(..))
import Resource.Styles
import ResourceVersion.Models as Models exposing (Model)
import ResourceVersion.Styles
import Routes
import SideBar.SideBar as SideBar
import StrictEvents
import Svg
import Svg.Attributes as SvgAttributes
import Time
import UpdateMsg exposing (UpdateMsg)
import UserState exposing (UserState(..))
import Views.DictView as DictView
import Views.Icon as Icon
import Views.Spinner as Spinner
import Views.Styles
import Views.TopBar as TopBar


type alias Flags =
    Concourse.VersionedResourceIdentifier


init : Flags -> ( Model, List Effect )
init flags =
    let
        model =
            { resourceVersionIdentifier = flags
            , pageStatus = Err Models.Empty
            , checkStatus = Models.CheckingSuccessfully
            , checkError = ""
            , checkSetupError = ""
            , lastChecked = Nothing
            , pinnedVersion = NotPinned
            , version = Nothing
            , now = Nothing
            , pinCommentLoading = False
            , textAreaFocused = False
            , isUserMenuExpanded = False
            , icon = Nothing
            , timeZone = Time.utc
            }
    in
    ( model
    , [ FetchResource { teamName = flags.teamName, pipelineName = flags.pipelineName, resourceName = flags.resourceName }
      , FetchResourceVersion flags
      , GetCurrentTimeZone
      , FetchPipelines
      ]
    )


changeToResourceVersion : Flags -> ET Model
changeToResourceVersion flags ( model, effects ) =
    ( { model
        | version = Nothing
      }
    , effects ++ [ FetchResourceVersion flags ]
    )


updatePinnedVersion : Concourse.Resource -> Model -> Model
updatePinnedVersion resource model =
    case ( resource.pinnedVersion, resource.pinnedInConfig ) of
        ( Nothing, _ ) ->
            case model.pinnedVersion of
                PinningTo _ ->
                    model

                _ ->
                    { model | pinnedVersion = NotPinned }

        ( Just v, True ) ->
            { model | pinnedVersion = PinnedStaticallyTo v }

        ( Just newVersion, False ) ->
            let
                pristineComment =
                    resource.pinComment |> Maybe.withDefault ""
            in
            case model.pinnedVersion of
                UnpinningFrom c _ ->
                    { model | pinnedVersion = UnpinningFrom c newVersion }

                PinnedDynamicallyTo { comment } _ ->
                    { model
                        | pinnedVersion =
                            PinnedDynamicallyTo
                                { comment = comment
                                , pristineComment = pristineComment
                                }
                                newVersion
                    }

                _ ->
                    { model
                        | pinnedVersion =
                            PinnedDynamicallyTo
                                { comment = pristineComment
                                , pristineComment = pristineComment
                                }
                                newVersion
                    }


getUpdateMessage : Model -> UpdateMsg
getUpdateMessage model =
    if model.pageStatus == Err Models.NotFound then
        UpdateMsg.NotFound

    else
        UpdateMsg.AOK


subscriptions : List Subscription
subscriptions =
    [ OnClockTick Subscription.FiveSeconds
    , OnClockTick Subscription.OneSecond
    , OnKeyDown
    , OnKeyUp
    ]


handleCallback : Callback -> Session -> ET Model
handleCallback callback session ( model, effects ) =
    case callback of
        GotCurrentTimeZone zone ->
            ( { model | timeZone = zone }, effects )

        ResourceFetched (Ok resource) ->
            ( { model
                | pageStatus = Ok ()
                , checkStatus =
                    if resource.failingToCheck then
                        Models.FailingToCheck

                    else
                        Models.CheckingSuccessfully
                , checkError = resource.checkError
                , checkSetupError = resource.checkSetupError
                , lastChecked = resource.lastChecked
                , icon = resource.icon
              }
                |> updatePinnedVersion resource
            , effects
                ++ (case resource.icon of
                        Just icon ->
                            [ RenderSvgIcon icon ]

                        Nothing ->
                            []
                   )
            )

        ResourceFetched (Err err) ->
            case err of
                Http.BadStatus { status } ->
                    if status.code == 401 then
                        ( model, effects ++ [ RedirectToLogin ] )

                    else if status.code == 404 then
                        ( { model | pageStatus = Err Models.NotFound }, effects )

                    else
                        ( model, effects )

                _ ->
                    ( model, effects )

        VersionedResourceFetched (Ok versionResource) ->
            let
                enabledStateAccordingToServer =
                    if versionResource.enabled then
                        Models.Enabled

                    else
                        Models.Disabled

                version =
                    { id =
                        { teamName = model.resourceVersionIdentifier.teamName
                        , pipelineName = model.resourceVersionIdentifier.pipelineName
                        , resourceName = model.resourceVersionIdentifier.resourceName
                        , versionID = versionResource.id
                        }
                    , version = versionResource.version
                    , metadata = versionResource.metadata
                    , enabled = enabledStateAccordingToServer
                    , expanded = True
                    , inputTo = []
                    , outputOf = []
                    , showTooltip = False
                    }
            in
            ( { model | version = Just version }
            , effects
                ++ [ FetchInputTo model.resourceVersionIdentifier
                   , FetchOutputOf model.resourceVersionIdentifier
                   ]
            )

        InputToFetched (Ok ( _, builds )) ->
            ( updateVersion (\v -> { v | inputTo = builds }) model
            , effects
            )

        OutputOfFetched (Ok ( _, builds )) ->
            ( updateVersion (\v -> { v | outputOf = builds }) model
            , effects
            )

        VersionPinned (Ok ()) ->
            case ( session.userState, model.now, model.pinnedVersion ) of
                ( UserStateLoggedIn user, Just time, PinningTo _ ) ->
                    let
                        commentText =
                            "pinned by "
                                ++ Login.userDisplayName user
                                ++ " at "
                                ++ formatDate model.timeZone time
                    in
                    ( { model
                        | pinnedVersion =
                            model.version
                                |> Maybe.map .version
                                |> Maybe.map
                                    (PinnedDynamicallyTo
                                        { comment = commentText
                                        , pristineComment = ""
                                        }
                                    )
                                |> Maybe.withDefault NotPinned
                      }
                    , effects
                        ++ [ SetPinComment
                                { teamName = model.resourceVersionIdentifier.teamName
                                , pipelineName = model.resourceVersionIdentifier.pipelineName
                                , resourceName = model.resourceVersionIdentifier.resourceName
                                }
                                commentText
                           ]
                    )

                _ ->
                    ( model, effects )

        VersionPinned (Err _) ->
            ( { model | pinnedVersion = NotPinned }
            , effects
            )

        VersionUnpinned (Ok ()) ->
            ( { model | pinnedVersion = NotPinned }
            , effects ++ [ FetchResourceVersion model.resourceVersionIdentifier ]
            )

        VersionUnpinned (Err _) ->
            ( { model | pinnedVersion = Pinned.quitUnpinning model.pinnedVersion }
            , effects
            )

        VersionToggled action _ result ->
            let
                newEnabledState : Models.VersionEnabledState
                newEnabledState =
                    case ( result, action ) of
                        ( Ok (), Message.Enable ) ->
                            Models.Enabled

                        ( Ok (), Message.Disable ) ->
                            Models.Disabled

                        ( Err _, Message.Enable ) ->
                            Models.Disabled

                        ( Err _, Message.Disable ) ->
                            Models.Enabled
            in
            ( updateVersion (\v -> { v | enabled = newEnabledState }) model
            , effects
            )

        Checked (Ok ()) ->
            ( { model | checkStatus = Models.CheckingSuccessfully }
            , effects
                ++ [ FetchResourceVersion model.resourceVersionIdentifier
                   ]
            )

        Checked (Err err) ->
            ( { model | checkStatus = Models.FailingToCheck }
            , case err of
                Http.BadStatus { status } ->
                    if status.code == 401 then
                        effects ++ [ RedirectToLogin ]

                    else
                        effects ++ [ FetchResourceVersion model.resourceVersionIdentifier ]

                _ ->
                    effects
            )

        CommentSet result ->
            ( { model
                | pinCommentLoading = False
                , pinnedVersion =
                    case ( result, model.pinnedVersion ) of
                        ( Ok (), PinnedDynamicallyTo { comment } v ) ->
                            PinnedDynamicallyTo
                                { comment = comment
                                , pristineComment = comment
                                }
                                v

                        ( _, pv ) ->
                            pv
              }
            , effects ++ [ FetchResourceVersion model.resourceVersionIdentifier ]
            )

        _ ->
            ( model, effects )


handleDelivery : Delivery -> ET Model
handleDelivery delivery ( model, effects ) =
    case delivery of
        ClockTicked OneSecond time ->
            ( { model | now = Just time }, effects )

        ClockTicked FiveSeconds _ ->
            ( model
            , effects
                ++ [ FetchResourceVersion model.resourceVersionIdentifier
                   , FetchPipelines
                   , FetchInputTo model.resourceVersionIdentifier
                   , FetchOutputOf model.resourceVersionIdentifier
                   ]
            )

        _ ->
            ( model, effects )


update : Message -> ET Model
update msg ( model, effects ) =
    case msg of
        Click (PinButton versionID) ->
            let
                version : Maybe Models.Version
                version =
                    model.version
            in
            case model.pinnedVersion of
                PinnedDynamicallyTo _ _ ->
                    ( { model
                        | pinnedVersion =
                            Pinned.startUnpinning model.pinnedVersion
                      }
                    , effects
                        ++ [ DoUnpinVersion
                                { teamName = model.resourceVersionIdentifier.teamName
                                , pipelineName = model.resourceVersionIdentifier.pipelineName
                                , resourceName = model.resourceVersionIdentifier.resourceName
                                }
                           ]
                    )

                NotPinned ->
                    ( { model
                        | pinnedVersion =
                            Pinned.startPinningTo versionID model.pinnedVersion
                      }
                    , case version of
                        Just _ ->
                            effects ++ [ DoPinVersion versionID ]

                        Nothing ->
                            effects
                    )

                _ ->
                    ( model, effects )

        Click PinIcon ->
            case model.pinnedVersion of
                PinnedDynamicallyTo _ _ ->
                    ( { model
                        | pinnedVersion =
                            Pinned.startUnpinning model.pinnedVersion
                      }
                    , effects
                        ++ [ DoUnpinVersion
                                { teamName = model.resourceVersionIdentifier.teamName
                                , pipelineName = model.resourceVersionIdentifier.pipelineName
                                , resourceName = model.resourceVersionIdentifier.resourceName
                                }
                           ]
                    )

                _ ->
                    ( model, effects )

        Click (VersionToggle versionID) ->
            let
                enabledState : Maybe Models.VersionEnabledState
                enabledState =
                    model.version
                        |> Maybe.map .enabled
            in
            case enabledState of
                Just Models.Enabled ->
                    ( updateVersion
                        (\v ->
                            { v | enabled = Models.Changing }
                        )
                        model
                    , effects ++ [ DoToggleVersion Message.Disable versionID ]
                    )

                Just Models.Disabled ->
                    ( updateVersion
                        (\v ->
                            { v | enabled = Models.Changing }
                        )
                        model
                    , effects ++ [ DoToggleVersion Message.Enable versionID ]
                    )

                _ ->
                    ( model, effects )

        Click (CheckButton isAuthorized) ->
            if isAuthorized then
                ( { model | checkStatus = Models.CurrentlyChecking }
                , effects
                    ++ [ DoCheck
                            { teamName = model.resourceVersionIdentifier.teamName
                            , pipelineName = model.resourceVersionIdentifier.pipelineName
                            , resourceName = model.resourceVersionIdentifier.resourceName
                            }
                       ]
                )

            else
                ( model, effects ++ [ RedirectToLogin ] )

        EditComment input ->
            let
                newPinnedVersion =
                    case model.pinnedVersion of
                        PinnedDynamicallyTo { pristineComment } v ->
                            PinnedDynamicallyTo
                                { comment = input
                                , pristineComment = pristineComment
                                }
                                v

                        x ->
                            x
            in
            ( { model | pinnedVersion = newPinnedVersion }, effects )

        Click SaveCommentButton ->
            case model.pinnedVersion of
                PinnedDynamicallyTo commentState _ ->
                    ( { model | pinCommentLoading = True }
                    , effects
                        ++ [ SetPinComment
                                { teamName = model.resourceVersionIdentifier.teamName
                                , pipelineName = model.resourceVersionIdentifier.pipelineName
                                , resourceName = model.resourceVersionIdentifier.resourceName
                                }
                                commentState.comment
                           ]
                    )

                _ ->
                    ( model, effects )

        FocusTextArea ->
            ( { model | textAreaFocused = True }, effects )

        BlurTextArea ->
            ( { model | textAreaFocused = False }, effects )

        _ ->
            ( model, effects )


updateVersion :
    (Models.Version -> Models.Version)
    -> Model
    -> Model
updateVersion updateFunc model =
    let
        newVersion =
            model.version
                |> Maybe.map updateFunc
    in
    { model | version = newVersion }


documentTitle : Model -> String
documentTitle model =
    model.resourceVersionIdentifier.resourceName ++ " #" ++ String.fromInt model.resourceVersionIdentifier.versionID


view : Session -> Model -> Html Message
view session model =
    let
        route =
            Routes.ResourceVersion model.resourceVersionIdentifier
    in
    Html.div
        (id "page-including-top-bar" :: Views.Styles.pageIncludingTopBar)
        [ Html.div
            (id "top-bar-app" :: Views.Styles.topBar False)
            [ SideBar.hamburgerMenu session
            , TopBar.concourseLogo
            , TopBar.breadcrumbs route
            , Login.view session.userState model False
            ]
        , Html.div
            (id "page-below-top-bar" :: Views.Styles.pageBelowTopBar route)
            [ SideBar.view session
                (Just
                    { pipelineName = model.resourceVersionIdentifier.pipelineName
                    , teamName = model.resourceVersionIdentifier.teamName
                    }
                )
            , if model.pageStatus == Err Models.Empty then
                Html.text ""

              else
                Html.div
                    [ style "flex-grow" "1"
                    , style "display" "flex"
                    , style "flex-direction" "column"
                    ]
                    [ header session model
                    , body session model
                    , commentBar session model
                    ]
            ]
        ]


header : { a | hovered : Maybe DomID } -> Model -> Html Message
header session model =
    let
        lastCheckedView =
            case ( model.now, model.lastChecked ) of
                ( Just now, Just date ) ->
                    viewLastChecked model.timeZone now date

                ( _, _ ) ->
                    Html.text ""

        iconView =
            case model.icon of
                Just icon ->
                    Svg.svg
                        [ style "height" "24px"
                        , style "width" "24px"
                        , style "margin-left" "-6px"
                        , style "margin-right" "10px"
                        , SvgAttributes.fill "white"
                        ]
                        [ Svg.use [ SvgAttributes.xlinkHref ("#" ++ icon ++ "-svg-icon") ] []
                        ]

                Nothing ->
                    Html.text ""
    in
    Html.div
        (id "page-header" :: Resource.Styles.headerBar)
        [ Html.h1
            Resource.Styles.headerResourceName
            [ iconView
            , Html.text model.resourceVersionIdentifier.resourceName
            ]
        , Html.div
            Resource.Styles.headerLastCheckedSection
            [ lastCheckedView ]
        , pinBar session model
        ]


body :
    { a | userState : UserState, hovered : Maybe DomID }
    -> Model
    -> Html Message
body session model =
    let
        sectionModel =
            { checkStatus = model.checkStatus
            , checkSetupError = model.checkSetupError
            , checkError = model.checkError
            , hovered = session.hovered
            , userState = session.userState
            , teamName = model.resourceVersionIdentifier.teamName
            }
    in
    Html.div
        (id "body" :: Resource.Styles.body)
        [ checkSection sectionModel
        , case model.version of
            Just version ->
                viewVersionedResource version model.pinnedVersion session.hovered

            Nothing ->
                Html.div [] []
        ]


checkSection :
    { a
        | checkStatus : Models.CheckStatus
        , checkSetupError : String
        , checkError : String
        , hovered : Maybe DomID
        , userState : UserState
        , teamName : String
    }
    -> Html Message
checkSection ({ checkStatus, checkSetupError, checkError } as model) =
    let
        failingToCheck =
            checkStatus == Models.FailingToCheck

        checkMessage =
            case checkStatus of
                Models.FailingToCheck ->
                    "checking failed"

                Models.CurrentlyChecking ->
                    "currently checking"

                Models.CheckingSuccessfully ->
                    "checking successfully"

        stepBody =
            if failingToCheck then
                if not (String.isEmpty checkSetupError) then
                    [ Html.div [ class "step-body" ]
                        [ Html.pre [] [ Html.text checkSetupError ]
                        ]
                    ]

                else
                    [ Html.div [ class "step-body" ]
                        [ Html.pre [] [ Html.text checkError ]
                        ]
                    ]

            else
                []

        statusIcon =
            case checkStatus of
                Models.CurrentlyChecking ->
                    Spinner.spinner
                        { sizePx = 14
                        , margin = "7px"
                        }

                _ ->
                    Icon.icon
                        { sizePx = 28
                        , image =
                            if failingToCheck then
                                "ic-exclamation-triangle.svg"

                            else
                                "ic-success-check.svg"
                        }
                        Resource.Styles.checkStatusIcon

        statusBar =
            Html.div
                Resource.Styles.checkBarStatus
                [ Html.h3 [] [ Html.text checkMessage ]
                , statusIcon
                ]

        checkBar =
            Html.div
                [ style "display" "flex" ]
                [ checkButton model
                , statusBar
                ]
    in
    Html.div [ class "resource-check-status" ] <| checkBar :: stepBody


checkButton :
    { a
        | hovered : Maybe DomID
        , userState : UserState
        , teamName : String
        , checkStatus : Models.CheckStatus
    }
    -> Html Message
checkButton ({ hovered, userState, checkStatus } as params) =
    let
        isMember =
            UserState.isMember params

        isHovered =
            hovered == Just (CheckButton isMember)

        isCurrentlyChecking =
            checkStatus == Models.CurrentlyChecking

        isAnonymous =
            UserState.isAnonymous userState

        isClickable =
            (isAnonymous || isMember)
                && not isCurrentlyChecking

        isHighlighted =
            (isClickable && isHovered) || isCurrentlyChecking
    in
    Html.div
        ([ onMouseEnter <| Hover <| Just <| CheckButton isMember
         , onMouseLeave <| Hover Nothing
         ]
            ++ Resource.Styles.checkButton isClickable
            ++ (if isClickable then
                    [ onClick <| Click <| CheckButton isMember ]

                else
                    []
               )
        )
        [ Icon.icon
            { sizePx = 20
            , image = "baseline-refresh-24px.svg"
            }
            (Resource.Styles.checkButtonIcon isHighlighted)
        ]


commentBar :
    { a | userState : UserState, hovered : Maybe DomID }
    ->
        { b
            | pinnedVersion : Models.PinnedVersion
            , resourceVersionIdentifier : Concourse.VersionedResourceIdentifier
            , pinCommentLoading : Bool
        }
    -> Html Message
commentBar { userState, hovered } { resourceVersionIdentifier, pinnedVersion, pinCommentLoading } =
    case pinnedVersion of
        PinnedDynamicallyTo commentState v ->
            let
                version =
                    viewVersion
                        [ Html.Attributes.style "align-self" "center" ]
                        v
            in
            Html.div
                (id "comment-bar" :: Resource.Styles.commentBar)
                [ Html.div Resource.Styles.commentBarContent <|
                    let
                        commentBarHeader =
                            Html.div
                                Resource.Styles.commentBarHeader
                                [ Html.div
                                    Resource.Styles.commentBarIconContainer
                                    [ Icon.icon
                                        { sizePx = 24
                                        , image = "baseline-message.svg"
                                        }
                                        Resource.Styles.commentBarMessageIcon
                                    , Icon.icon
                                        { sizePx = 20
                                        , image = "pin-ic-white.svg"
                                        }
                                        Resource.Styles.commentBarPinIcon
                                    ]
                                , version
                                ]
                    in
                    if
                        UserState.isMember
                            { teamName = resourceVersionIdentifier.teamName
                            , userState = userState
                            }
                    then
                        [ commentBarHeader
                        , Html.textarea
                            ([ onInput EditComment
                             , value commentState.comment
                             , placeholder "enter a comment"
                             , onFocus FocusTextArea
                             , onBlur BlurTextArea
                             ]
                                ++ Resource.Styles.commentTextArea
                            )
                            []
                        , Html.button
                            (let
                                commentChanged =
                                    commentState.comment
                                        /= commentState.pristineComment
                             in
                             [ onMouseEnter <| Hover <| Just SaveCommentButton
                             , onMouseLeave <| Hover Nothing
                             , onClick <| Click SaveCommentButton
                             ]
                                ++ Resource.Styles.commentSaveButton
                                    { isHovered =
                                        not pinCommentLoading
                                            && commentChanged
                                            && hovered
                                            == Just SaveCommentButton
                                    , commentChanged = commentChanged
                                    }
                            )
                            (if pinCommentLoading then
                                [ Spinner.spinner
                                    { sizePx = 12
                                    , margin = "0"
                                    }
                                ]

                             else
                                [ Html.text "save" ]
                            )
                        ]

                    else
                        [ commentBarHeader
                        , Html.pre
                            Resource.Styles.commentText
                            [ Html.text commentState.pristineComment ]
                        , Html.div [ style "height" "24px" ] []
                        ]
                ]

        _ ->
            Html.text ""


pinBar :
    { a | hovered : Maybe DomID }
    -> { b | pinnedVersion : Models.PinnedVersion }
    -> Html Message
pinBar { hovered } { pinnedVersion } =
    let
        pinBarVersion =
            Pinned.stable pinnedVersion

        attrList : List ( Html.Attribute Message, Bool ) -> List (Html.Attribute Message)
        attrList =
            List.filter Tuple.second >> List.map Tuple.first

        isPinnedStatically =
            case pinnedVersion of
                PinnedStaticallyTo _ ->
                    True

                _ ->
                    False

        isPinnedDynamically =
            case pinnedVersion of
                PinnedDynamicallyTo _ _ ->
                    True

                _ ->
                    False
    in
    Html.div
        (attrList
            [ ( id "pin-bar", True )
            , ( onMouseEnter <| Hover <| Just PinBar, isPinnedStatically )
            , ( onMouseLeave <| Hover Nothing, isPinnedStatically )
            ]
            ++ Resource.Styles.pinBar (ME.isJust pinBarVersion)
        )
        (Icon.icon
            { sizePx = 25
            , image =
                if ME.isJust pinBarVersion then
                    "pin-ic-white.svg"

                else
                    "pin-ic-grey.svg"
            }
            (attrList
                [ ( id "pin-icon", True )
                , ( onClick <| Click PinIcon
                  , isPinnedDynamically
                  )
                , ( onMouseEnter <| Hover <| Just PinIcon
                  , isPinnedDynamically
                  )
                , ( onMouseLeave <| Hover Nothing, True )
                ]
                ++ Resource.Styles.pinIcon
                    { isPinnedDynamically = isPinnedDynamically
                    , hover = hovered == Just PinIcon
                    }
            )
            :: (case pinBarVersion of
                    Just v ->
                        [ viewVersion [] v ]

                    _ ->
                        []
               )
            ++ (if hovered == Just PinBar then
                    [ Html.div
                        (id "pin-bar-tooltip" :: Resource.Styles.pinBarTooltip)
                        [ Html.text "pinned in pipeline config" ]
                    ]

                else
                    []
               )
        )


viewVersionedResource :
    Models.Version
    -> Models.PinnedVersion
    -> Maybe DomID
    -> Html Message
viewVersionedResource version pinnedVersion hovered =
    let
        pinState =
            case Pinned.pinState version.version version.id pinnedVersion of
                PinnedStatically _ ->
                    PinnedStatically version.showTooltip

                x ->
                    x
    in
    Html.ul [ class "list list-collapsable list-enableDisable resource-versions" ]
        [ Html.li
            (case ( pinState, version.enabled ) of
                ( Disabled, _ ) ->
                    [ style "opacity" "0.5" ]

                ( _, Models.Disabled ) ->
                    [ style "opacity" "0.5" ]

                _ ->
                    []
            )
            (Html.div
                [ style "display" "flex"
                , style "margin" "5px 0px"
                ]
                [ viewEnabledCheckbox
                    { enabled = version.enabled
                    , id = version.id
                    , pinState = pinState
                    }
                , viewPinButton
                    { versionID = version.id
                    , pinState = pinState
                    , hovered = hovered
                    }
                , viewVersionHeader
                    { id = version.id
                    , version = version.version
                    , pinnedState = pinState
                    }
                ]
                :: (if version.expanded then
                        [ viewVersionBody
                            { inputTo = version.inputTo
                            , outputOf = version.outputOf
                            , metadata = version.metadata
                            }
                        ]

                    else
                        []
                   )
            )
        ]


viewVersionBody :
    { a
        | inputTo : List Concourse.Build
        , outputOf : List Concourse.Build
        , metadata : Concourse.Metadata
    }
    -> Html Message
viewVersionBody { inputTo, outputOf, metadata } =
    Html.div
        [ style "display" "flex"
        , style "padding" "5px 10px"
        ]
        [ Html.div [ class "vri" ] <|
            List.concat
                [ [ Html.div [ style "line-height" "25px" ] [ Html.text "inputs to" ] ]
                , viewBuilds <| listToMap inputTo
                ]
        , Html.div [ class "vri" ] <|
            List.concat
                [ [ Html.div [ style "line-height" "25px" ] [ Html.text "outputs of" ] ]
                , viewBuilds <| listToMap outputOf
                ]
        , Html.div [ class "vri metadata-container" ]
            [ Html.div [ class "list-collapsable-title" ] [ Html.text "metadata" ]
            , viewMetadata metadata
            ]
        ]


viewEnabledCheckbox :
    { a
        | enabled : Models.VersionEnabledState
        , id : Models.VersionId
        , pinState : VersionPinState
    }
    -> Html Message
viewEnabledCheckbox ({ enabled, id } as params) =
    let
        clickHandler =
            case enabled of
                Models.Enabled ->
                    [ onClick <| Click <| VersionToggle id ]

                Models.Changing ->
                    []

                Models.Disabled ->
                    [ onClick <| Click <| VersionToggle id ]
    in
    Html.div
        (Html.Attributes.attribute "aria-label" "Toggle Resource Version Enabled"
            :: ResourceVersion.Styles.enabledCheckbox params
            ++ clickHandler
        )
        (case enabled of
            Models.Enabled ->
                []

            Models.Changing ->
                [ Spinner.spinner
                    { sizePx = 12.5
                    , margin = "6.25px"
                    }
                ]

            Models.Disabled ->
                []
        )


viewPinButton :
    { versionID : Models.VersionId
    , pinState : VersionPinState
    , hovered : Maybe DomID
    }
    -> Html Message
viewPinButton { versionID, pinState, hovered } =
    let
        eventHandlers =
            case pinState of
                Enabled ->
                    [ onClick <| Click <| PinButton versionID ]

                PinnedDynamically ->
                    [ onClick <| Click <| PinButton versionID ]

                PinnedStatically _ ->
                    [ onMouseOver <| Hover <| Just <| PinButton versionID
                    , onMouseOut <| Hover Nothing
                    ]

                Disabled ->
                    []

                InTransition ->
                    []
    in
    Html.div
        (Html.Attributes.attribute "aria-label" "Pin Resource Version"
            :: Resource.Styles.pinButton pinState
            ++ eventHandlers
        )
        (case pinState of
            PinnedStatically _ ->
                if hovered == Just (PinButton versionID) then
                    [ Html.div
                        Resource.Styles.pinButtonTooltip
                        [ Html.text "enable via pipeline config" ]
                    ]

                else
                    []

            InTransition ->
                [ Spinner.spinner
                    { sizePx = 12.5
                    , margin = "6.25px"
                    }
                ]

            _ ->
                []
        )


viewVersionHeader :
    { a
        | id : Models.VersionId
        , version : Concourse.Version
        , pinnedState : VersionPinState
    }
    -> Html Message
viewVersionHeader { id, version, pinnedState } =
    Html.div
        ((onClick <| Click <| VersionHeader id)
            :: Resource.Styles.versionHeader pinnedState
        )
        [ viewVersion [] version ]


viewVersion : List (Html.Attribute Message) -> Concourse.Version -> Html Message
viewVersion attrs version =
    version
        |> Dict.map (always Html.text)
        |> DictView.view attrs


viewMetadata : Concourse.Metadata -> Html Message
viewMetadata metadata =
    Html.dl [ class "build-metadata" ]
        (List.concatMap viewMetadataField metadata)


viewMetadataField : Concourse.MetadataField -> List (Html a)
viewMetadataField field =
    [ Html.dt [] [ Html.text field.name ]
    , Html.dd []
        [ Html.pre [ class "metadata-field" ] [ Html.text field.value ]
        ]
    ]


listToMap : List Concourse.Build -> Dict.Dict String (List Concourse.Build)
listToMap builds =
    let
        insertBuild =
            \build dict ->
                let
                    jobName =
                        case build.job of
                            Nothing ->
                                -- Jobless builds shouldn't appear on this page!
                                ""

                            Just job ->
                                job.jobName

                    oldList =
                        Dict.get jobName dict

                    newList =
                        case oldList of
                            Nothing ->
                                [ build ]

                            Just list ->
                                list ++ [ build ]
                in
                Dict.insert jobName newList dict
    in
    List.foldr insertBuild Dict.empty builds


viewBuilds : Dict.Dict String (List Concourse.Build) -> List (Html Message)
viewBuilds buildDict =
    List.concatMap (viewBuildsByJob buildDict) <| Dict.keys buildDict


formatDate : Time.Zone -> Time.Posix -> String
formatDate =
    DateFormat.format
        [ DateFormat.monthNameAbbreviated
        , DateFormat.text " "
        , DateFormat.dayOfMonthNumber
        , DateFormat.text " "
        , DateFormat.yearNumber
        , DateFormat.text " "
        , DateFormat.hourFixed
        , DateFormat.text ":"
        , DateFormat.minuteFixed
        , DateFormat.text ":"
        , DateFormat.secondFixed
        , DateFormat.text " "
        , DateFormat.amPmUppercase
        ]


viewLastChecked : Time.Zone -> Time.Posix -> Time.Posix -> Html a
viewLastChecked timeZone now date =
    let
        ago =
            Duration.between date now
    in
    Html.table [ id "last-checked" ]
        [ Html.tr
            []
            [ Html.td [] [ Html.text "checked" ]
            , Html.td
                [ title <| formatDate timeZone date ]
                [ Html.span [] [ Html.text (Duration.format ago ++ " ago") ] ]
            ]
        ]


viewBuildsByJob : Dict.Dict String (List Concourse.Build) -> String -> List (Html Message)
viewBuildsByJob buildDict jobName =
    let
        oneBuildToLi =
            \build ->
                case build.job of
                    Nothing ->
                        Html.li [ class <| Concourse.BuildStatus.show build.status ]
                            [ Html.text <| "#" ++ build.name ]

                    Just job ->
                        let
                            link =
                                Routes.Build
                                    { id =
                                        { teamName = job.teamName
                                        , pipelineName = job.pipelineName
                                        , jobName = job.jobName
                                        , buildName = build.name
                                        }
                                    , highlight = Routes.HighlightNothing
                                    }
                        in
                        Html.li [ class <| Concourse.BuildStatus.show build.status ]
                            [ Html.a
                                [ StrictEvents.onLeftClick <| GoToRoute link
                                , href (Routes.toString link)
                                ]
                                [ Html.text <| "#" ++ build.name ]
                            ]
    in
    [ Html.h3 [ class "man pas ansi-bright-black-bg" ] [ Html.text jobName ]
    , Html.ul [ class "builds-list" ]
        (case Dict.get jobName buildDict of
            Nothing ->
                []

            -- never happens
            Just buildList ->
                List.map oneBuildToLi buildList
        )
    ]
